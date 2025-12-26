use anyhow::Result;
use crossterm::{
    event::{self, DisableMouseCapture, EnableMouseCapture, Event, KeyCode},
    execute,
    terminal::{disable_raw_mode, enable_raw_mode, EnterAlternateScreen, LeaveAlternateScreen},
};
use ratatui::{
    backend::{Backend, CrosstermBackend},
    layout::{Constraint, Direction, Layout},
    style::{Color, Modifier, Style},
    text::{Span, Line},
    widgets::{Block, Borders, List, ListItem, Paragraph},
    Terminal,
};
use std::{io, time::{Duration, Instant}};

// Data structures for UI
struct AppState {
    rx_packets: u64,
    rx_bytes: u64,
    tx_packets: u64,
    tx_bytes: u64,
    events: Vec<String>,
}

trait DataProvider {
    fn update(&mut self, state: &mut AppState) -> Result<()>;
}

// -----------------------------------------------------------------------------
// Real Data Provider (Linux only) - Reads Pinned Maps
#[cfg(target_os = "linux")]
use aya::maps::{MapData, PerCpuArray};

#[cfg(target_os = "linux")]
use crate::ebpf::PacketCounters;

#[cfg(target_os = "linux")]
struct RealDataProvider {
    counters: PerCpuArray<MapData, PacketCounters>,
    // Track last values to show delta/rates
    last_counters: PacketCounters,
}

#[cfg(target_os = "linux")]
impl RealDataProvider {
    fn new() -> Result<Self> {
        use std::path::Path;
        
        let pin_path = Path::new("/sys/fs/bpf/sennet/counters");
        if !pin_path.exists() {
            anyhow::bail!("Pinned map not found at {:?}. Is the agent running?", pin_path);
        }
        
        // Use MapData::from_pin (not Map::from_pin) in aya 0.12
        let map_data = MapData::from_pin(pin_path)?;
        let counters: PerCpuArray<_, PacketCounters> = PerCpuArray::try_from(map_data)?;
        
        Ok(Self { 
            counters,
            last_counters: PacketCounters::default(),
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
        let delta_rx = current.rx_packets.saturating_sub(self.last_counters.rx_packets);
        if delta_rx > 1000 && state.events.len() < 20 {
            state.events.insert(0, format!("High RX rate: {} pkts/250ms", delta_rx));
        }
        
        self.last_counters = current;
        Ok(())
    }
}

// -----------------------------------------------------------------------------
// Mock Data Provider (Windows / Dev)
struct MockDataProvider {
    start_time: Instant,
}

impl MockDataProvider {
    fn new() -> Self {
        Self { start_time: Instant::now() }
    }
}

impl DataProvider for MockDataProvider {
    fn update(&mut self, state: &mut AppState) -> Result<()> {
        let elapsed = self.start_time.elapsed().as_secs_f64();
        
        // Simulate traffic patterns (sine wave)
        let rate_rx = (elapsed.sin() * 500.0 + 1000.0) as u64; // packets/sec
        let rate_tx = (elapsed.cos() * 200.0 + 500.0) as u64;
        
        state.rx_packets += rate_rx;
        state.rx_bytes += rate_rx * 128; // avg 128 bytes
        state.tx_packets += rate_tx;
        state.tx_bytes += rate_tx * 128;

        // Simulate events
        if rand::random::<u8>() > 250 {
           state.events.insert(0, format!("[{:.0}s] Large Packet: 192.168.1.5 -> 10.0.0.1 (Proto 6)", elapsed));
           if state.events.len() > 20 { state.events.pop(); }
        }
        
        Ok(())
    }
}

// -----------------------------------------------------------------------------
// Main Run Function

pub fn run() -> Result<()> {
    // Setup terminal
    enable_raw_mode()?;
    let mut stdout = io::stdout();
    execute!(stdout, EnterAlternateScreen, EnableMouseCapture)?;
    let backend = CrosstermBackend::new(stdout);
    let mut terminal = Terminal::new(backend)?;

    // Create App State
    let mut app_state = AppState {
        rx_packets: 0,
        rx_bytes: 0,
        tx_packets: 0,
        tx_bytes: 0,
        events: Vec::new(),
    };

    // Choose Provider
    #[cfg(target_os = "linux")]
    let mut provider: Box<dyn DataProvider> = match RealDataProvider::new() {
        Ok(real) => Box::new(real),
        Err(_) => Box::new(MockDataProvider::new()), // Fallback to mock if real fails
    };

    #[cfg(not(target_os = "linux"))]
    let mut provider: Box<dyn DataProvider> = Box::new(MockDataProvider::new());

    // Run Loop
    let res = run_app(&mut terminal, &mut *provider, &mut app_state);

    // Restore terminal
    disable_raw_mode()?;
    execute!(
        terminal.backend_mut(),
        LeaveAlternateScreen,
        DisableMouseCapture
    )?;
    terminal.show_cursor()?;

    if let Err(err) = res {
        println!("{:?}", err);
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
                if let KeyCode::Char('q') = key.code {
                    return Ok(());
                }
            }
        }

        if last_tick.elapsed() >= tick_rate {
            provider.update(state)?;
            last_tick = Instant::now();
        }
    }
}

fn ui(f: &mut ratatui::Frame, state: &AppState) {
    let chunks = Layout::default()
        .direction(Direction::Vertical)
        .margin(1)
        .constraints(
            [
                Constraint::Length(3), // Header
                Constraint::Length(8), // Stats
                Constraint::Min(0),    // Events
            ]
            .as_ref(),
        )
        .split(f.area());

    // 1. Header
    let title = Paragraph::new(Span::styled(
        "Sennet Network Monitor (Press 'q' to quit)",
        Style::default().fg(Color::Cyan).add_modifier(Modifier::BOLD),
    ))
    .block(Block::default().borders(Borders::ALL));
    f.render_widget(title, chunks[0]);

    // 2. Stats
    let stats_text = vec![
        Line::from(vec![
            Span::raw("RX Packets: "),
            Span::styled(format!("{}", state.rx_packets), Style::default().fg(Color::Green)),
        ]),
        Line::from(vec![
            Span::raw("RX Bytes:   "),
            Span::styled(format!("{}", state.rx_bytes), Style::default().fg(Color::Green)),
        ]),
        Line::from(vec![
            Span::raw("TX Packets: "),
            Span::styled(format!("{}", state.tx_packets), Style::default().fg(Color::Blue)),
        ]),
        Line::from(vec![
            Span::raw("TX Bytes:   "),
            Span::styled(format!("{}", state.tx_bytes), Style::default().fg(Color::Blue)),
        ]),
    ];
    let stats = Paragraph::new(stats_text)
        .block(Block::default().title("Traffic Stats").borders(Borders::ALL));
    f.render_widget(stats, chunks[1]);

    // 3. Events
    let events: Vec<ListItem> = state
        .events
        .iter()
        .map(|e| ListItem::new(Span::raw(e)))
        .collect();
    let events_list = List::new(events)
        .block(Block::default().title("Recent Events").borders(Borders::ALL));
    f.render_widget(events_list, chunks[2]);
}
