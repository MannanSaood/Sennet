use anyhow::Result;
#[cfg(target_os = "linux")]
use crossterm::{
    event::EnableMouseCapture,
    terminal::{enable_raw_mode, EnterAlternateScreen},
};
use crossterm::{
    event::{self, DisableMouseCapture, Event, KeyCode},
    execute,
    terminal::{disable_raw_mode, LeaveAlternateScreen},
};
#[cfg(target_os = "linux")]
use ratatui::backend::CrosstermBackend;
use ratatui::{
    backend::Backend,
    layout::{Constraint, Direction, Layout},
    style::{Color, Modifier, Style},
    text::{Line, Span},
    widgets::{Block, Borders, List, ListItem, Paragraph, Sparkline, Tabs},
    Terminal,
};
use std::{
    io,
    time::{Duration, Instant},
};

// Data structures for UI
struct AppState {
    tab: usize,
    rx_rate: u64,
    tx_rate: u64,
    history: Vec<u64>,
    paused: bool,
    rx_packets: u64,
    rx_bytes: u64,
    tx_packets: u64,
    tx_bytes: u64,
    events: Vec<String>,
    drop_events: Vec<DropEventDisplay>, // Phase 6.3: Drop events panel
}

/// Display-ready drop event
#[derive(Clone)]
struct DropEventDisplay {
    timestamp_secs: u64,
    reason: String,
    hook: Option<String>, // From netfilter if available
    severity: DropSeverity,
}

#[derive(Clone, Copy)]
enum DropSeverity {
    Security, // Red - netfilter drops, socket filters
    Config,   // Yellow - policy drops, routing issues
    Normal,   // Gray - TCP retransmits, etc.
}

trait DataProvider {
    fn update(&mut self, state: &mut AppState) -> Result<()>;
}

// -----------------------------------------------------------------------------
// Real Data Provider (Linux only) - Reads Pinned Maps
#[cfg(target_os = "linux")]
use aya::maps::{Map, MapData, PerCpuArray, RingBuf};

#[cfg(target_os = "linux")]
use crate::ebpf::{
    drop_reason_str, nf_hook_str, nf_verdict_str, DropEvent, NetfilterEvent, PacketCounters,
};

#[cfg(target_os = "linux")]
struct RealDataProvider {
    counters: PerCpuArray<MapData, PacketCounters>,
    drop_events_rb: Option<RingBuf<MapData>>,
    nf_events_rb: Option<RingBuf<MapData>>, // Phase 6.2: Netfilter events
    // Track last values to show delta/rates
    last_counters: PacketCounters,
    start_time: Instant,
}

#[cfg(target_os = "linux")]
impl RealDataProvider {
    fn new() -> Result<Self> {
        use std::path::Path;

        let pin_path = Path::new("/sys/fs/bpf/sennet/counters");
        if !pin_path.exists() {
            anyhow::bail!(
                "Pinned map not found at {:?}. Is the agent running?",
                pin_path
            );
        }

        // In aya 0.12: MapData::from_pin -> Map::PerCpuArray -> PerCpuArray::try_from(Map)
        let map_data = MapData::from_pin(pin_path)?;
        let map = Map::PerCpuArray(map_data);
        let counters: PerCpuArray<_, PacketCounters> = map.try_into()?;

        // Try to open DROP_EVENTS RingBuf (Phase 6.1)
        let drop_events_rb = {
            let drop_path = Path::new("/sys/fs/bpf/sennet/drop_events");
            if drop_path.exists() {
                match MapData::from_pin(drop_path) {
                    Ok(data) => {
                        let map = Map::RingBuf(data);
                        match map.try_into() {
                            Ok(rb) => Some(rb),
                            Err(_) => None,
                        }
                    }
                    Err(_) => None,
                }
            } else {
                None
            }
        };

        // Try to open NF_EVENTS RingBuf (Phase 6.2)
        let nf_events_rb = {
            let nf_path = Path::new("/sys/fs/bpf/sennet/nf_events");
            if nf_path.exists() {
                match MapData::from_pin(nf_path) {
                    Ok(data) => {
                        let map = Map::RingBuf(data);
                        match map.try_into() {
                            Ok(rb) => Some(rb),
                            Err(_) => None,
                        }
                    }
                    Err(_) => None,
                }
            } else {
                None
            }
        };

        Ok(Self {
            counters,
            drop_events_rb,
            nf_events_rb,
            last_counters: PacketCounters::default(),
            start_time: Instant::now(),
        })
    }

    fn read_totals(&self) -> Result<PacketCounters> {
        let mut total = PacketCounters::default();

        // Read ingress counters (index 0)
        if let Ok(values) = self.counters.get(&0, 0) {
            for cpu_val in values.iter() {
                total.rx_packets += cpu_val.rx_packets;
                total.rx_bytes += cpu_val.rx_bytes;
                total.drop_count += cpu_val.drop_count;
            }
        }

        // Read egress counters (index 1)
        if let Ok(values) = self.counters.get(&1, 0) {
            for cpu_val in values.iter() {
                total.tx_packets += cpu_val.tx_packets;
                total.tx_bytes += cpu_val.tx_bytes;
            }
        }

        Ok(total)
    }

    fn poll_drop_events(&mut self, state: &mut AppState) {
        // Poll kfree_skb drop events (Phase 6.1)
        if let Some(ref mut rb) = self.drop_events_rb {
            while let Some(item) = rb.next() {
                if item.len() >= std::mem::size_of::<DropEvent>() {
                    let event: DropEvent =
                        unsafe { std::ptr::read_unaligned(item.as_ptr() as *const DropEvent) };

                    let elapsed_secs = self.start_time.elapsed().as_secs();
                    let reason_str = drop_reason_str(event.reason);

                    let severity = match event.reason {
                        7 => DropSeverity::Security, // NETFILTER_DROP
                        5 => DropSeverity::Security, // SOCKET_FILTER
                        2 => DropSeverity::Config,   // NO_SOCKET
                        37 => DropSeverity::Config,  // IP_OUTNOROUTES
                        _ => DropSeverity::Normal,
                    };

                    let display = DropEventDisplay {
                        timestamp_secs: elapsed_secs,
                        reason: reason_str.to_string(),
                        hook: None,
                        severity,
                    };

                    state.drop_events.insert(0, display);
                    if state.drop_events.len() > 20 {
                        state.drop_events.pop();
                    }
                }
            }
        }

        // Poll netfilter events (Phase 6.2)
        if let Some(ref mut rb) = self.nf_events_rb {
            while let Some(item) = rb.next() {
                if item.len() >= std::mem::size_of::<NetfilterEvent>() {
                    let event: NetfilterEvent =
                        unsafe { std::ptr::read_unaligned(item.as_ptr() as *const NetfilterEvent) };

                    // Only show DROP verdicts (verdict == 0)
                    if event.verdict == 0 {
                        let elapsed_secs = self.start_time.elapsed().as_secs();
                        let hook_name = nf_hook_str(event.hook);
                        let verdict_name = nf_verdict_str(event.verdict);

                        let display = DropEventDisplay {
                            timestamp_secs: elapsed_secs,
                            reason: format!("NF_{}", verdict_name),
                            hook: Some(hook_name.to_string()),
                            severity: DropSeverity::Security, // Netfilter drops are security-relevant
                        };

                        state.drop_events.insert(0, display);
                        if state.drop_events.len() > 20 {
                            state.drop_events.pop();
                        }
                    }
                }
            }
        }
    }
}

#[cfg(target_os = "linux")]
impl DataProvider for RealDataProvider {
    fn update(&mut self, state: &mut AppState) -> Result<()> {
        let current = self.read_totals()?;

        // Update state with current totals
        state.rx_packets = current.rx_packets;
        state.rx_bytes = current.rx_bytes;
        state.tx_packets = current.tx_packets;
        state.tx_bytes = current.tx_bytes;

        // Add event if significant traffic delta detected
        let delta_rx = current
            .rx_packets
            .saturating_sub(self.last_counters.rx_packets);
        if delta_rx > 1000 && state.events.len() < 20 {
            state
                .events
                .insert(0, format!("High RX rate: {} pkts/250ms", delta_rx));
        }

        // Poll drop events from RingBuf
        self.poll_drop_events(state);

        self.last_counters = current;
        Ok(())
    }
}

// -----------------------------------------------------------------------------
// Main Run Function

#[cfg(not(target_os = "linux"))]
pub fn run() -> Result<()> {
    anyhow::bail!("Live monitoring requires Linux and a running Sennet eBPF agent")
}

// Restores the terminal on normal return, setup errors, and panic unwinding.
struct TerminalGuard;
impl Drop for TerminalGuard {
    fn drop(&mut self) {
        let _ = disable_raw_mode();
        let _ = execute!(
            io::stdout(),
            LeaveAlternateScreen,
            DisableMouseCapture,
            crossterm::cursor::Show
        );
    }
}

#[cfg(target_os = "linux")]
pub fn run() -> Result<()> {
    use std::io::IsTerminal;
    ensure_tty(io::stdout().is_terminal())?;
    // Resolve the source before entering raw mode. Never disguise collection
    // failure as healthy, simulated traffic.
    #[cfg(target_os = "linux")]
    let mut provider: Box<dyn DataProvider> = Box::new(RealDataProvider::new()?);

    // Setup terminal
    enable_raw_mode()?;
    let _terminal_guard = TerminalGuard;
    let mut stdout = io::stdout();
    execute!(stdout, EnterAlternateScreen, EnableMouseCapture)?;
    let backend = CrosstermBackend::new(stdout);
    let mut terminal = Terminal::new(backend)?;

    // Create App State
    let mut app_state = AppState {
        tab: 0,
        rx_rate: 0,
        tx_rate: 0,
        history: Vec::new(),
        paused: false,
        rx_packets: 0,
        rx_bytes: 0,
        tx_packets: 0,
        tx_bytes: 0,
        events: Vec::new(),
        drop_events: Vec::new(),
    };

    provider.update(&mut app_state)?;
    // Run Loop
    let res = run_app(&mut terminal, &mut *provider, &mut app_state);

    res
}

fn ensure_tty(is_terminal: bool) -> Result<()> {
    if !is_terminal {
        anyhow::bail!("interactive TUI requires a TTY; use `sennet top --json` for automation");
    }
    Ok(())
}

fn run_app<B: Backend>(
    terminal: &mut Terminal<B>,
    provider: &mut dyn DataProvider,
    state: &mut AppState,
) -> Result<()> {
    let mut last_tick = Instant::now();
    let tick_rate = Duration::from_millis(250);

    loop {
        terminal.draw(|f| ui(f, state))?;

        let timeout = tick_rate
            .checked_sub(last_tick.elapsed())
            .unwrap_or_else(|| Duration::from_secs(0));

        if crossterm::event::poll(timeout)? {
            if let Event::Key(key) = event::read()? {
                if key.code == KeyCode::Char('q')
                    || key.code == KeyCode::Esc
                    || (key.code == KeyCode::Char('c')
                        && key
                            .modifiers
                            .contains(crossterm::event::KeyModifiers::CONTROL))
                {
                    return Ok(());
                }
                if key.code == KeyCode::Char('p') {
                    state.paused = !state.paused;
                }
                match key.code {
                    KeyCode::Tab | KeyCode::Right | KeyCode::Char('l') => {
                        state.tab = (state.tab + 1) % 4
                    }
                    KeyCode::BackTab | KeyCode::Left | KeyCode::Char('h') => {
                        state.tab = (state.tab + 3) % 4
                    }
                    KeyCode::Char(c @ '1'..='4') => state.tab = c as usize - '1' as usize,
                    _ => {}
                }
            }
        }

        if last_tick.elapsed() >= tick_rate {
            if !state.paused {
                let previous_rx = state.rx_bytes;
                let previous_tx = state.tx_bytes;
                let elapsed = last_tick.elapsed().as_secs_f64().max(0.001);
                provider.update(state)?;
                state.rx_rate =
                    (state.rx_bytes.saturating_sub(previous_rx) as f64 / elapsed) as u64;
                state.tx_rate =
                    (state.tx_bytes.saturating_sub(previous_tx) as f64 / elapsed) as u64;
                state
                    .history
                    .push(state.rx_rate.saturating_add(state.tx_rate));
                if state.history.len() > 120 {
                    state.history.remove(0);
                }
            }
            last_tick = Instant::now();
        }
    }
}

fn ui(f: &mut ratatui::Frame, state: &AppState) {
    if f.area().width < 45 || f.area().height < 15 {
        f.render_widget(
            Paragraph::new("Resize terminal to at least 45×15. q: quit"),
            f.area(),
        );
        return;
    }
    let regions = Layout::default()
        .direction(Direction::Vertical)
        .margin(1)
        .constraints([
            Constraint::Length(3),
            Constraint::Length(3),
            Constraint::Length(5),
            Constraint::Min(1),
            Constraint::Length(1),
        ])
        .split(f.area());
    f.render_widget(
        Paragraph::new(format!(
            " SENNET  /  NETWORK INVESTIGATION     {}",
            if state.paused { "PAUSED" } else { "LOCAL" }
        ))
        .style(
            Style::default()
                .fg(Color::LightGreen)
                .add_modifier(Modifier::BOLD),
        )
        .block(Block::default().borders(Borders::BOTTOM)),
        regions[0],
    );
    f.render_widget(
        Tabs::new(["Overview", "Flows", "Drops", "Exporter"])
            .select(state.tab)
            .highlight_style(
                Style::default()
                    .fg(Color::LightGreen)
                    .add_modifier(Modifier::BOLD),
            ),
        regions[1],
    );
    let counters = Layout::default()
        .direction(Direction::Horizontal)
        .constraints([Constraint::Percentage(50), Constraint::Percentage(50)])
        .split(regions[2]);
    for (area, label, rate, total, color) in [
        (
            counters[0],
            " INBOUND ",
            state.rx_rate,
            state.rx_bytes,
            Color::LightGreen,
        ),
        (
            counters[1],
            " OUTBOUND ",
            state.tx_rate,
            state.tx_bytes,
            Color::LightBlue,
        ),
    ] {
        f.render_widget(
            Paragraph::new(vec![
                Line::from(Span::styled(
                    format!(" {} /s", human_bytes(rate)),
                    Style::default().fg(color).add_modifier(Modifier::BOLD),
                )),
                Line::from(format!(" {} observed", human_bytes(total))),
            ])
            .block(Block::default().title(label).borders(Borders::ALL)),
            area,
        );
    }
    let items: Vec<ListItem> = if state.drop_events.is_empty() {
        vec![ListItem::new(
            " Drop collection unavailable: layout-dependent probes were removed.",
        )]
    } else {
        state
            .drop_events
            .iter()
            .map(|e| {
                ListItem::new(format!(
                    " {}s  {}  {}",
                    e.timestamp_secs,
                    e.reason,
                    e.hook.as_deref().unwrap_or("")
                ))
                .style(Style::default().fg(match e.severity {
                    DropSeverity::Security => Color::LightRed,
                    DropSeverity::Config => Color::Yellow,
                    DropSeverity::Normal => Color::Gray,
                }))
            })
            .collect()
    };
    match state.tab {
        0 => f.render_widget(Sparkline::default().data(&state.history).style(Style::default().fg(Color::LightGreen)).block(Block::default().title(" TRAFFIC HISTORY · bytes/s ").borders(Borders::ALL)), regions[3]),
        1 => f.render_widget(Paragraph::new("Flow attribution unavailable: non-portable socket probes were removed.").block(Block::default().title(" FLOWS ").borders(Borders::ALL)), regions[3]),
        2 => f.render_widget(List::new(items).block(Block::default().title(" DROPS ").borders(Borders::ALL)), regions[3]),
        _ => f.render_widget(Paragraph::new("Exporter health is persisted in exporter-health.json and included in --json output.").block(Block::default().title(" EXPORTER ").borders(Borders::ALL)), regions[3]),
    }
    f.render_widget(
        Paragraph::new(" 1-4 / ←→ tabs    q / Esc quit    p pause    Refresh 250ms")
            .style(Style::default().fg(Color::DarkGray)),
        regions[4],
    );
}
fn human_bytes(value: u64) -> String {
    if value >= 1024 * 1024 * 1024 {
        format!("{:.2} GiB", value as f64 / (1024.0 * 1024.0 * 1024.0))
    } else if value >= 1024 * 1024 {
        format!("{:.2} MiB", value as f64 / (1024.0 * 1024.0))
    } else if value >= 1024 {
        format!("{:.1} KiB", value as f64 / 1024.0)
    } else {
        format!("{} B", value)
    }
}

pub fn snapshot_json() -> Result<()> {
    #[cfg(target_os = "linux")]
    {
        let provider = RealDataProvider::new()?;
        let totals = provider.read_totals()?;
        let config = crate::config::Config::load()?;
        let health_path = config.state_dir.join("exporter-health.json");
        let exporter: serde_json::Value =
            serde_json::from_slice(&std::fs::read(&health_path).map_err(|error| {
                anyhow::anyhow!(
                    "exporter health unavailable at {}: {}",
                    health_path.display(),
                    error
                )
            })?)?;
        println!(
            "{}",
            serde_json::json!({"collection":{"status":"collecting","rx_bytes":totals.rx_bytes.to_string(),"tx_bytes":totals.tx_bytes.to_string(),"rx_packets":totals.rx_packets.to_string(),"tx_packets":totals.tx_packets.to_string()},"exporter":exporter,"flows":{"status":"unsupported"},"drops":{"status":"unsupported"}})
        );
        Ok(())
    }
    #[cfg(not(target_os = "linux"))]
    {
        anyhow::bail!("Local eBPF snapshots require Linux");
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn non_tty_is_rejected_with_json_guidance() {
        let error = ensure_tty(false).unwrap_err();
        assert!(error.to_string().contains("--json"));
    }

    #[test]
    fn tty_is_accepted() {
        assert!(ensure_tty(true).is_ok());
    }
}
