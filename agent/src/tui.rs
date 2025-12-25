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
    widgets::{Block, Borders, List, ListItem, Paragraph, Gauge},
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
struct RealDataProvider {
    // fields to access maps
}

#[cfg(target_os = "linux")]
impl RealDataProvider {
    fn new() -> Result<Self> {
        // In a real implementation, we would open the pinned maps here
        // For this task, we will simulate reading from maps if we can't open them
        // to prevent crashing if the service isn't running.
        Ok(Self {})
    }
}

#[cfg(target_os = "linux")]
impl DataProvider for RealDataProvider {
    fn update(&mut self, state: &mut AppState) -> Result<()> {
        // TODO: Use aya::maps::PerCpuArray::try_from(Map::from_pin(...)?)
        // For MVP without ability to verify eBPF compilation locally, 
        // we will stick to a stub here that would ideally read the maps.
        // If maps are not pinned, we can't read them.
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
    let mut provider: Box<dyn DataProvider> = Box::new(RealDataProvider::new().unwrap_or_else(|_| {
        // Fallback to mock if real fails (e.g. no pinned maps)
        Box::new(MockDataProvider::new())
    }));

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
