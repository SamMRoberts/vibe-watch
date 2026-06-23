//! Interactive terminal dashboard for [`SessionAnalytics`], built on ratatui.
//!
//! [`run`] drives a full-screen interactive view. [`render`] is a pure drawing
//! function (no terminal side effects) so it can be exercised with ratatui's
//! `TestBackend` in unit tests.

use std::io::{self, Stdout};
use std::path::PathBuf;
use std::sync::mpsc;
use std::time::{Duration, Instant};

use anyhow::Result;
use ratatui::backend::CrosstermBackend;
use ratatui::crossterm::{
    event::{self, Event, KeyCode, KeyEventKind},
    execute,
    terminal::{disable_raw_mode, enable_raw_mode, EnterAlternateScreen, LeaveAlternateScreen},
};
use ratatui::layout::{Constraint, Layout, Margin, Rect};
use ratatui::style::{Color, Modifier, Style};
use ratatui::text::{Line, Span};
use ratatui::widgets::{
    Block, Cell, Gauge, Paragraph, Row, Scrollbar, ScrollbarOrientation, ScrollbarState, Table,
};
use ratatui::{Frame, Terminal};
use tachyonfx::{fx, EffectManager, Interpolation};

use crate::analytics::{CreditSource, RatesSource, SessionAnalytics, TurnMetrics};
use crate::cache::{CacheConfig, SessionCache};
use crate::session_scan::{
    self, FormatFilter, LoadedSession, ScanEvent, ScanProgress, SessionLoadError,
};

/// View state shared between the event loop and [`render`].
#[derive(Debug, Default, Clone, Copy)]
pub struct ViewState {
    /// Index of the currently highlighted turn.
    pub selected: usize,
    /// First visible row in the activity usage table.
    pub activity_scroll: usize,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum BrowserView {
    Repositories,
    Sessions,
    SessionDetail,
}

#[derive(Debug, Clone)]
pub struct BrowserState {
    pub view: BrowserView,
    pub selected_repo: usize,
    pub selected_session: usize,
    pub detail: ViewState,
    progress: ScanProgress,
    repositories: Vec<RepositoryGroup>,
}

impl BrowserState {
    pub fn new(progress: ScanProgress) -> Self {
        Self {
            view: BrowserView::Repositories,
            selected_repo: 0,
            selected_session: 0,
            detail: ViewState::default(),
            progress,
            repositories: Vec::new(),
        }
    }

    pub fn add_loaded(&mut self, session: LoadedSession) {
        let repository = session
            .analytics
            .repository_path
            .clone()
            .and_then(|path| {
                let trimmed = path.trim();
                (!trimmed.is_empty()).then(|| trimmed.to_string())
            })
            .unwrap_or_else(|| "(unknown repository)".to_string());
        self.group_mut(repository)
            .entries
            .push(SessionEntry::Loaded(Box::new(session)));
        self.clamp_selection();
    }

    pub fn add_error(&mut self, error: SessionLoadError) {
        self.group_mut("(errors)".to_string())
            .entries
            .push(SessionEntry::Error(error));
        self.clamp_selection();
    }

    pub fn set_progress(&mut self, progress: ScanProgress) {
        self.progress = progress;
    }

    pub fn apply_scan_event(&mut self, event: ScanEvent) {
        match event {
            ScanEvent::Progress(progress) => self.set_progress(progress),
            ScanEvent::Loaded(session) => self.add_loaded(*session),
            ScanEvent::Error(error) => self.add_error(error),
            ScanEvent::Finished => {
                self.progress.finished = true;
            }
        }
    }

    fn select_first(&mut self) {
        match self.view {
            BrowserView::Repositories => {
                self.selected_repo = 0;
                self.selected_session = 0;
                self.detail = ViewState::default();
            }
            BrowserView::Sessions => {
                self.selected_session = 0;
                self.detail = ViewState::default();
            }
            BrowserView::SessionDetail => {
                self.detail.selected = 0;
                self.detail.activity_scroll = 0;
            }
        }
    }

    fn select_last(&mut self) {
        match self.view {
            BrowserView::Repositories => {
                if !self.repositories.is_empty() {
                    self.selected_repo = self.repositories.len() - 1;
                    self.selected_session = 0;
                    self.detail = ViewState::default();
                }
            }
            BrowserView::Sessions => {
                if let Some(group) = self.selected_group() {
                    if !group.entries.is_empty() {
                        self.selected_session = group.entries.len() - 1;
                        self.detail = ViewState::default();
                    }
                }
            }
            BrowserView::SessionDetail => {
                if let Some(session) = self.selected_loaded_session() {
                    if !session.analytics.turns.is_empty() {
                        self.detail.selected = session.analytics.turns.len() - 1;
                        self.detail.activity_scroll = 0;
                    }
                }
            }
        }
    }

    fn select_next(&mut self) {
        match self.view {
            BrowserView::Repositories => {
                if !self.repositories.is_empty() {
                    self.selected_repo = (self.selected_repo + 1).min(self.repositories.len() - 1);
                    self.selected_session = 0;
                    self.detail = ViewState::default();
                }
            }
            BrowserView::Sessions => {
                if let Some(group) = self.selected_group() {
                    if !group.entries.is_empty() {
                        self.selected_session =
                            (self.selected_session + 1).min(group.entries.len() - 1);
                        self.detail = ViewState::default();
                    }
                }
            }
            BrowserView::SessionDetail => self.select_next_turn(),
        }
    }

    fn select_previous(&mut self) {
        match self.view {
            BrowserView::Repositories => {
                self.selected_repo = self.selected_repo.saturating_sub(1);
                self.selected_session = 0;
                self.detail = ViewState::default();
            }
            BrowserView::Sessions => {
                self.selected_session = self.selected_session.saturating_sub(1);
                self.detail = ViewState::default();
            }
            BrowserView::SessionDetail => {
                let next = self.detail.selected.saturating_sub(1);
                if next != self.detail.selected {
                    self.detail.selected = next;
                    self.detail.activity_scroll = 0;
                }
            }
        }
    }

    fn drill_in(&mut self) {
        match self.view {
            BrowserView::Repositories => {
                if self.selected_group().is_some() {
                    self.view = BrowserView::Sessions;
                    self.selected_session = 0;
                    self.detail = ViewState::default();
                }
            }
            BrowserView::Sessions => {
                if self.selected_loaded_session().is_some() {
                    self.view = BrowserView::SessionDetail;
                    self.detail = ViewState::default();
                }
            }
            BrowserView::SessionDetail => {}
        }
    }

    fn go_back(&mut self) -> bool {
        match self.view {
            BrowserView::Repositories => false,
            BrowserView::Sessions => {
                self.view = BrowserView::Repositories;
                true
            }
            BrowserView::SessionDetail => {
                self.view = BrowserView::Sessions;
                true
            }
        }
    }

    fn select_next_turn(&mut self) {
        let Some(session) = self.selected_loaded_session() else {
            return;
        };
        let turn_count = session.analytics.turns.len();
        if turn_count == 0 {
            return;
        }
        let next = (self.detail.selected + 1).min(turn_count - 1);
        if next != self.detail.selected {
            self.detail.selected = next;
            self.detail.activity_scroll = 0;
        }
    }

    fn group_mut(&mut self, repository: String) -> &mut RepositoryGroup {
        if let Some(index) = self
            .repositories
            .iter()
            .position(|group| group.repository == repository)
        {
            return &mut self.repositories[index];
        }
        self.repositories.push(RepositoryGroup {
            repository,
            entries: Vec::new(),
        });
        self.repositories
            .last_mut()
            .expect("repository just pushed")
    }

    fn selected_group(&self) -> Option<&RepositoryGroup> {
        self.repositories.get(self.selected_repo)
    }

    fn selected_loaded_session(&self) -> Option<&LoadedSession> {
        let group = self.selected_group()?;
        match group.entries.get(self.selected_session)? {
            SessionEntry::Loaded(session) => Some(session.as_ref()),
            SessionEntry::Error(_) => None,
        }
    }

    fn clamp_selection(&mut self) {
        if self.repositories.is_empty() {
            self.selected_repo = 0;
            self.selected_session = 0;
            return;
        }
        self.selected_repo = self.selected_repo.min(self.repositories.len() - 1);
        let entry_count = self.repositories[self.selected_repo].entries.len();
        self.selected_session = if entry_count == 0 {
            0
        } else {
            self.selected_session.min(entry_count - 1)
        };
    }

    fn error_count(&self) -> usize {
        self.repositories
            .iter()
            .flat_map(|group| &group.entries)
            .filter(|entry| matches!(entry, SessionEntry::Error(_)))
            .count()
    }
}

#[derive(Debug, Clone)]
struct RepositoryGroup {
    repository: String,
    entries: Vec<SessionEntry>,
}

#[derive(Debug, Clone)]
enum SessionEntry {
    Loaded(Box<LoadedSession>),
    Error(SessionLoadError),
}

#[derive(Debug, Default)]
struct BrowserEffects {
    body: EffectManager<&'static str>,
    progress: EffectManager<&'static str>,
    enabled: bool,
}

impl BrowserEffects {
    fn new() -> Self {
        Self {
            enabled: motion_enabled(),
            ..Self::default()
        }
    }

    fn on_scan_event(&mut self, event: &ScanEvent) {
        if !self.enabled {
            return;
        }

        match event {
            ScanEvent::Error(_) => self.progress.add_unique_effect(
                "scan-error",
                fx::fade_from_fg(Color::DarkGray, (120, Interpolation::Linear)),
            ),
            ScanEvent::Loaded(_) | ScanEvent::Progress(_) | ScanEvent::Finished => {}
        }
    }

    fn on_navigation(&mut self) {
        if !self.enabled {
            return;
        }

        self.body.add_unique_effect(
            "view-transition",
            fx::fade_from_fg(Color::DarkGray, (180, Interpolation::SineOut)),
        );
    }

    fn is_running(&self) -> bool {
        self.enabled && (self.body.is_running() || self.progress.is_running())
    }

    fn process(&mut self, frame: &mut Frame, state: &BrowserState, elapsed: Duration) {
        if !self.is_running() {
            return;
        }

        let areas = browser_effect_areas(frame.area(), state.view);
        let elapsed = elapsed.into();
        self.body
            .process_effects(elapsed, frame.buffer_mut(), areas.body);
        self.progress
            .process_effects(elapsed, frame.buffer_mut(), areas.progress);
    }
}

#[derive(Debug, Clone, Copy)]
struct BrowserEffectAreas {
    body: Rect,
    progress: Rect,
}

fn motion_enabled() -> bool {
    std::env::var_os("VIBE_WATCH_NO_MOTION").is_none() && std::env::var_os("NO_COLOR").is_none()
}

fn browser_effect_areas(area: Rect, view: BrowserView) -> BrowserEffectAreas {
    if view == BrowserView::SessionDetail {
        let rows = Layout::vertical([
            Constraint::Length(6),
            Constraint::Min(6),
            Constraint::Length(4),
            Constraint::Length(3),
        ])
        .split(area);
        return BrowserEffectAreas {
            body: rows[1],
            progress: rows[3],
        };
    }

    let rows = Layout::vertical([
        Constraint::Length(4),
        Constraint::Min(6),
        Constraint::Length(3),
        Constraint::Length(3),
    ])
    .split(area);
    BrowserEffectAreas {
        body: rows[1],
        progress: rows[2],
    }
}

/// Launch the interactive dashboard, returning when the user quits.
pub fn run(analytics: &SessionAnalytics) -> Result<()> {
    let mut terminal = setup_terminal()?;
    let result = event_loop(&mut terminal, analytics);
    restore_terminal(&mut terminal)?;
    result
}

pub fn run_browser(
    path: Option<PathBuf>,
    filter: FormatFilter,
    cache_config: CacheConfig,
    cache: Option<SessionCache>,
) -> Result<()> {
    let mut terminal = setup_terminal()?;
    let result = browser_event_loop(&mut terminal, path, filter, cache_config, cache);
    restore_terminal(&mut terminal)?;
    result
}

fn setup_terminal() -> Result<Terminal<CrosstermBackend<Stdout>>> {
    enable_raw_mode()?;
    let mut stdout = io::stdout();
    execute!(stdout, EnterAlternateScreen)?;
    Ok(Terminal::new(CrosstermBackend::new(stdout))?)
}

fn restore_terminal(terminal: &mut Terminal<CrosstermBackend<Stdout>>) -> Result<()> {
    disable_raw_mode()?;
    execute!(terminal.backend_mut(), LeaveAlternateScreen)?;
    terminal.show_cursor()?;
    Ok(())
}

fn event_loop(
    terminal: &mut Terminal<CrosstermBackend<Stdout>>,
    analytics: &SessionAnalytics,
) -> Result<()> {
    let mut state = ViewState::default();
    let turn_count = analytics.turns.len();
    loop {
        terminal.draw(|frame| render(frame, analytics, &state))?;
        if let Event::Key(key) = event::read()? {
            if key.kind != KeyEventKind::Press {
                continue;
            }
            match key.code {
                KeyCode::Char('q') | KeyCode::Esc => break,
                KeyCode::Down | KeyCode::Char('j') if turn_count > 0 => {
                    let next = (state.selected + 1).min(turn_count - 1);
                    if next != state.selected {
                        state.selected = next;
                        state.activity_scroll = 0;
                    }
                }
                KeyCode::Up | KeyCode::Char('k') => {
                    let next = state.selected.saturating_sub(1);
                    if next != state.selected {
                        state.selected = next;
                        state.activity_scroll = 0;
                    }
                }
                KeyCode::PageDown | KeyCode::Char(']') => {
                    state.activity_scroll = state.activity_scroll.saturating_add(3);
                }
                KeyCode::PageUp | KeyCode::Char('[') => {
                    state.activity_scroll = state.activity_scroll.saturating_sub(3);
                }
                KeyCode::Home => {
                    state.selected = 0;
                    state.activity_scroll = 0;
                }
                KeyCode::End if turn_count > 0 => {
                    state.selected = turn_count - 1;
                    state.activity_scroll = 0;
                }
                _ => {}
            }
        }
    }
    Ok(())
}

fn browser_event_loop(
    terminal: &mut Terminal<CrosstermBackend<Stdout>>,
    path: Option<PathBuf>,
    filter: FormatFilter,
    cache_config: CacheConfig,
    mut cache: Option<SessionCache>,
) -> Result<()> {
    let (sender, receiver) = mpsc::channel();
    std::thread::spawn(move || {
        let result = session_scan::discover_sessions(path.as_deref(), filter);
        match result {
            Ok(candidates) => {
                if cache_config.enabled {
                    session_scan::scan_candidates_with_cache_handle(
                        candidates,
                        filter,
                        &cache_config,
                        cache.as_mut(),
                        |event| sender.send(event).is_ok(),
                    );
                } else {
                    session_scan::scan_candidates_with(candidates, filter, |event| {
                        sender.send(event).is_ok()
                    });
                }
            }
            Err(error) => {
                let _ = sender.send(ScanEvent::Error(SessionLoadError {
                    path: path.unwrap_or_else(|| PathBuf::from("(default roots)")),
                    modified: None,
                    message: error.to_string(),
                }));
                let _ = sender.send(ScanEvent::Progress(ScanProgress {
                    processed: 0,
                    total: 0,
                    finished: true,
                }));
                let _ = sender.send(ScanEvent::Finished);
            }
        }
    });

    let mut state = BrowserState::new(ScanProgress::default());
    let mut effects = BrowserEffects::new();
    let mut last_frame = Instant::now();
    loop {
        for event in receiver.try_iter() {
            effects.on_scan_event(&event);
            state.apply_scan_event(event);
        }

        let now = Instant::now();
        let elapsed = now.duration_since(last_frame);
        last_frame = now;
        terminal.draw(|frame| {
            render_browser(frame, &state);
            effects.process(frame, &state, elapsed);
        })?;

        let timeout = if effects.is_running() {
            Duration::from_millis(33)
        } else {
            Duration::from_millis(100)
        };
        if !event::poll(timeout)? {
            continue;
        }
        if let Event::Key(key) = event::read()? {
            if key.kind != KeyEventKind::Press {
                continue;
            }
            match key.code {
                KeyCode::Char('q') => break,
                KeyCode::Esc => {
                    if !state.go_back() {
                        break;
                    }
                    effects.on_navigation();
                }
                KeyCode::Down | KeyCode::Char('j') => state.select_next(),
                KeyCode::Up | KeyCode::Char('k') => state.select_previous(),
                KeyCode::Home => state.select_first(),
                KeyCode::End => state.select_last(),
                KeyCode::Enter | KeyCode::Right => {
                    let before = state.view;
                    state.drill_in();
                    if state.view != before {
                        effects.on_navigation();
                    }
                }
                KeyCode::Left | KeyCode::Backspace => {
                    if state.go_back() {
                        effects.on_navigation();
                    }
                }
                KeyCode::PageDown | KeyCode::Char(']')
                    if state.view == BrowserView::SessionDetail =>
                {
                    state.detail.activity_scroll = state.detail.activity_scroll.saturating_add(3);
                }
                KeyCode::PageUp | KeyCode::Char('[')
                    if state.view == BrowserView::SessionDetail =>
                {
                    state.detail.activity_scroll = state.detail.activity_scroll.saturating_sub(3);
                }
                _ => {}
            }
        }
    }

    Ok(())
}

/// Draw the full dashboard into `frame`. Pure with respect to the terminal.
pub fn render(frame: &mut Frame, analytics: &SessionAnalytics, state: &ViewState) {
    let rows = Layout::vertical([
        Constraint::Length(6),
        Constraint::Min(6),
        Constraint::Length(4),
        Constraint::Length(3),
    ])
    .split(frame.area());

    render_header(frame, rows[0], analytics);

    let body =
        Layout::horizontal([Constraint::Percentage(62), Constraint::Percentage(38)]).split(rows[1]);
    render_turns(frame, body[0], analytics, state);
    render_aggregates(frame, body[1], analytics, state);

    render_footer(frame, rows[2], analytics, state);
    render_timeline(frame, rows[3], analytics, state);
}

pub fn render_browser(frame: &mut Frame, state: &BrowserState) {
    if state.view == BrowserView::SessionDetail {
        if let Some(session) = state.selected_loaded_session() {
            render(frame, &session.analytics, &state.detail);
            return;
        }
    }

    let rows = Layout::vertical([
        Constraint::Length(4),
        Constraint::Min(6),
        Constraint::Length(3),
        Constraint::Length(3),
    ])
    .split(frame.area());

    render_browser_header(frame, rows[0], state);
    match state.view {
        BrowserView::Repositories | BrowserView::SessionDetail => {
            render_repositories(frame, rows[1], state)
        }
        BrowserView::Sessions => render_sessions(frame, rows[1], state),
    }
    render_progress(frame, rows[2], state);
    render_browser_footer(frame, rows[3], state);
}

fn render_browser_header(frame: &mut Frame, area: Rect, state: &BrowserState) {
    let loaded = state
        .repositories
        .iter()
        .flat_map(|group| &group.entries)
        .filter(|entry| matches!(entry, SessionEntry::Loaded(_)))
        .count();
    let lines = vec![
        Line::from(vec![
            Span::styled("Repositories ", Style::new().fg(Color::Cyan).bold()),
            Span::raw(format!("{}   ", state.repositories.len())),
            Span::styled("sessions ", Style::new().fg(Color::DarkGray)),
            Span::raw(loaded.to_string()),
            Span::raw(format!("   errors {}", state.error_count())),
        ]),
        Line::from(Span::styled(
            "Enter/Right drill in   Left/Backspace back   q quit",
            Style::new().fg(Color::DarkGray),
        )),
    ];
    frame.render_widget(
        Paragraph::new(lines).block(Block::bordered().title(" vibe-watch sessions ")),
        area,
    );
}

fn render_repositories(frame: &mut Frame, area: Rect, state: &BrowserState) {
    let rows = state.repositories.iter().enumerate().map(|(index, group)| {
        let selected = index == state.selected_repo;
        let style = selected_row_style(selected);
        Row::new(vec![
            Cell::from(selected_marker(selected)),
            Cell::from(group.repository.clone()),
            Cell::from(group.entries.len().to_string()),
            Cell::from(group_turn_count(group).to_string()),
            Cell::from(group_credit_summary(group)),
        ])
        .style(style)
    });

    let table = Table::new(
        rows,
        [
            Constraint::Length(2),
            Constraint::Percentage(56),
            Constraint::Length(10),
            Constraint::Length(8),
            Constraint::Length(18),
        ],
    )
    .header(
        Row::new(vec!["", "repository", "sessions", "turns", "AIC"])
            .style(Style::new().add_modifier(Modifier::BOLD)),
    )
    .block(Block::bordered().title(" Repositories "));
    frame.render_widget(table, area);
}

fn render_sessions(frame: &mut Frame, area: Rect, state: &BrowserState) {
    let Some(group) = state.selected_group() else {
        frame.render_widget(
            Paragraph::new("No repository selected").block(Block::bordered().title(" Sessions ")),
            area,
        );
        return;
    };

    let rows = group.entries.iter().enumerate().map(|(index, entry)| {
        let selected = index == state.selected_session;
        let style = selected_row_style(selected);
        match entry {
            SessionEntry::Loaded(session) => Row::new(vec![
                Cell::from(selected_marker(selected)),
                Cell::from(session_label(&session.analytics)),
                Cell::from(session.analytics.turn_count.to_string()),
                Cell::from(analytics_credit_summary(&session.analytics)),
                Cell::from(short_path(&session.path)),
                Cell::from("loaded"),
            ])
            .style(style),
            SessionEntry::Error(error) => Row::new(vec![
                Cell::from(selected_marker(selected)),
                Cell::from(short_path(&error.path)),
                Cell::from("-"),
                Cell::from("n/a"),
                Cell::from(error.message.clone()),
                Cell::from("error"),
            ])
            .style(style.fg(Color::Red)),
        }
    });

    let table = Table::new(
        rows,
        [
            Constraint::Length(2),
            Constraint::Percentage(28),
            Constraint::Length(7),
            Constraint::Length(18),
            Constraint::Percentage(38),
            Constraint::Length(8),
        ],
    )
    .header(
        Row::new(vec!["", "session", "turns", "AIC", "path/error", "status"])
            .style(Style::new().add_modifier(Modifier::BOLD)),
    )
    .block(Block::bordered().title(format!(" Sessions - {} ", group.repository)));
    frame.render_widget(table, area);
}

fn render_progress(frame: &mut Frame, area: Rect, state: &BrowserState) {
    let ratio = if state.progress.total == 0 {
        if state.progress.finished {
            0.0
        } else {
            1.0
        }
    } else {
        state.progress.processed as f64 / state.progress.total as f64
    };
    let label = progress_label(state);
    let gauge = Gauge::default()
        .block(Block::bordered().title(" Progress "))
        .gauge_style(Style::new().fg(Color::Green))
        .ratio(ratio.clamp(0.0, 1.0))
        .label(label);
    frame.render_widget(gauge, area);
}

fn progress_label(state: &BrowserState) -> String {
    if state.progress.finished && state.progress.total == 0 {
        return format!("No sessions found   errors {}", state.error_count());
    }

    let verb = if state.progress.finished {
        "Loaded"
    } else {
        "Loading"
    };
    format!(
        "{verb} {}/{}   errors {}",
        state.progress.processed,
        state.progress.total,
        state.error_count()
    )
}

fn render_browser_footer(frame: &mut Frame, area: Rect, state: &BrowserState) {
    let detail = match state.view {
        BrowserView::Repositories => "repository dashboard",
        BrowserView::Sessions => "repository sessions",
        BrowserView::SessionDetail => "session detail",
    };
    frame.render_widget(
        Paragraph::new(Line::from(Span::styled(
            detail,
            Style::new().fg(Color::Gray),
        )))
        .block(Block::bordered()),
        area,
    );
}

fn selected_row_style(selected: bool) -> Style {
    if selected {
        Style::new()
            .fg(Color::Black)
            .bg(Color::Cyan)
            .add_modifier(Modifier::BOLD)
    } else {
        Style::new()
    }
}

fn selected_marker(selected: bool) -> &'static str {
    if selected {
        ">"
    } else {
        ""
    }
}

fn group_turn_count(group: &RepositoryGroup) -> usize {
    group
        .entries
        .iter()
        .filter_map(|entry| match entry {
            SessionEntry::Loaded(session) => Some(session.analytics.turn_count),
            SessionEntry::Error(_) => None,
        })
        .sum()
}

fn group_credit_summary(group: &RepositoryGroup) -> String {
    let mut total = 0.0;
    let mut any = false;
    let mut source = None;
    for entry in &group.entries {
        let SessionEntry::Loaded(session) = entry else {
            continue;
        };
        if let Some(value) = analytics_credit_value(&session.analytics) {
            total += value;
            any = true;
            source.get_or_insert(session.analytics.credit_source);
        }
    }
    if any {
        format!(
            "{total:.1} {}",
            credit_source_label(source.unwrap_or(CreditSource::Unknown))
        )
    } else {
        "n/a".to_string()
    }
}

fn analytics_credit_summary(analytics: &SessionAnalytics) -> String {
    analytics_credit_value(analytics).map_or_else(
        || "n/a".to_string(),
        |value| {
            format!(
                "{value:.1} {}",
                credit_source_label(analytics.credit_source)
            )
        },
    )
}

fn analytics_credit_value(analytics: &SessionAnalytics) -> Option<f64> {
    match (analytics.credit_source, analytics.total_credits) {
        (CreditSource::Reported | CreditSource::Estimated | CreditSource::Mixed, Some(value)) => {
            Some(value)
        }
        (CreditSource::OutputOnly, _) => Some(analytics.total_output_credits),
        _ => None,
    }
}

fn credit_source_label(source: CreditSource) -> &'static str {
    match source {
        CreditSource::Reported => "reported",
        CreditSource::Estimated => "estimated",
        CreditSource::Mixed => "mixed",
        CreditSource::OutputOnly => "output-only",
        CreditSource::Unknown => "unknown",
    }
}

fn session_label(analytics: &SessionAnalytics) -> String {
    analytics
        .title
        .clone()
        .or_else(|| analytics.session_id.clone())
        .unwrap_or_else(|| "(unknown session)".to_string())
}

fn short_path(path: &std::path::Path) -> String {
    let parts: Vec<String> = path
        .components()
        .rev()
        .take(2)
        .map(|component| component.as_os_str().to_string_lossy().into_owned())
        .collect();
    parts.into_iter().rev().collect::<Vec<_>>().join("/")
}

fn render_header(frame: &mut Frame, area: Rect, analytics: &SessionAnalytics) {
    let model = format!(
        "{} ({})",
        analytics.model_name.as_deref().unwrap_or("?"),
        analytics.model_id.as_deref().unwrap_or("?")
    );
    let rates = match (analytics.rates_source, analytics.rates) {
        (RatesSource::Unknown, _) | (_, None) => "rates: unknown".to_string(),
        (_, Some(rates)) => format!(
            "rates AIC/1M: in {:.0} out {:.0} cache {:.0}",
            rates.input_per_m, rates.output_per_m, rates.cache_per_m
        ),
    };
    let tokens = token_summary(analytics);
    let credits = credit_summary(analytics);

    let lines = vec![
        Line::from(vec![
            Span::styled("Session ", Style::new().fg(Color::DarkGray)),
            Span::raw(
                analytics
                    .session_id
                    .as_deref()
                    .unwrap_or("(unknown)")
                    .to_string(),
            ),
        ]),
        Line::from(vec![
            Span::styled("Repo    ", Style::new().fg(Color::DarkGray)),
            Span::raw(
                analytics
                    .repository_path
                    .as_deref()
                    .unwrap_or("(unknown)")
                    .to_string(),
            ),
        ]),
        Line::from(vec![
            Span::styled("Model   ", Style::new().fg(Color::DarkGray)),
            Span::styled(model, Style::new().fg(Color::Cyan)),
            Span::raw("   "),
            Span::styled(rates, Style::new().fg(Color::DarkGray)),
        ]),
        Line::from(vec![
            Span::styled("Totals  ", Style::new().fg(Color::DarkGray)),
            Span::raw(format!("{} turns   ", analytics.turn_count)),
            Span::styled(tokens, Style::new().fg(Color::Green)),
            Span::raw("   "),
            Span::styled(credits, Style::new().fg(Color::Yellow)),
            Span::raw(format!(
                "   model {:.0}s",
                analytics.total_elapsed_ms as f64 / 1000.0
            )),
        ]),
    ];

    let block = Block::bordered()
        .title(" vibe-watch ")
        .title_style(Style::new().bold());
    frame.render_widget(Paragraph::new(lines).block(block), area);
}

fn token_summary(analytics: &SessionAnalytics) -> String {
    format!(
        "in {}   out {}   cached {}",
        token_value(analytics.total_input_tokens),
        analytics.total_output_tokens,
        token_value(analytics.total_cached_tokens)
    )
}

fn token_value(value: Option<u64>) -> String {
    value.map_or_else(|| "n/a".to_string(), |tokens| tokens.to_string())
}

fn credit_summary(analytics: &SessionAnalytics) -> String {
    match (analytics.credit_source, analytics.total_credits) {
        (CreditSource::Reported, Some(total)) => format!("{total:.1} AIC reported"),
        (CreditSource::Estimated, Some(total)) => format!("{total:.1} AIC estimated"),
        (CreditSource::Mixed, Some(total)) => format!("{total:.1} AIC mixed"),
        (CreditSource::OutputOnly, _) => {
            format!("{:.1} AIC output-only", analytics.total_output_credits)
        }
        (CreditSource::Unknown, _) => "AIC n/a".to_string(),
        (_, None) => "AIC n/a".to_string(),
    }
}


fn render_turns(frame: &mut Frame, area: Rect, analytics: &SessionAnalytics, state: &ViewState) {
    let max_tokens = analytics
        .turns
        .iter()
        .map(|t| t.output_tokens)
        .max()
        .unwrap_or(0);
    let visible_rows = turn_visible_rows(area);
    let selected = selected_turn_index(analytics, state);
    let start = visible_window_start(analytics.turns.len(), selected, visible_rows);
    let end = (start + visible_rows).min(analytics.turns.len());

    let rows = analytics.turns[start..end].iter().map(|turn| {
        let bar = bar_string(turn.output_tokens, max_tokens, 14);
        let is_selected = turn.index == selected;
        let style = turn_row_style(turn.index, is_selected);
        Row::new(vec![
            Cell::from(turn.index.to_string()),
            Cell::from(short_id(turn.request_id.as_deref())),
            Cell::from(token_value(turn.input_tokens)),
            Cell::from(turn.output_tokens.to_string()),
            Cell::from(turn.credit_display()),
            Cell::from(format!("{:.1}%", turn.pct_output_tokens)),
            Cell::from(Span::styled(bar, Style::new().fg(Color::Green))),
        ])
        .style(style)
    });

    let widths = [
        Constraint::Length(3),
        Constraint::Length(12),
        Constraint::Length(8),
        Constraint::Length(8),
        Constraint::Length(6),
        Constraint::Length(6),
        Constraint::Min(14),
    ];
    let header = Row::new(vec![
        "#",
        "request",
        "in_tok",
        "out_tok",
        "AIC",
        "%tok",
        "output share",
    ])
    .style(Style::new().add_modifier(Modifier::BOLD));
    let table = Table::new(rows, widths)
        .header(header)
        .block(Block::bordered().title(" Turns "));
    frame.render_widget(table, area);

    if visible_rows > 0 && analytics.turns.len() > visible_rows {
        let mut scrollbar_state = ScrollbarState::new(analytics.turns.len())
            .position(start)
            .viewport_content_length(visible_rows);
        let scrollbar = Scrollbar::new(ScrollbarOrientation::VerticalRight)
            .begin_symbol(Some("↑"))
            .end_symbol(Some("↓"))
            .thumb_style(Style::new().fg(Color::Cyan))
            .track_style(Style::new().fg(Color::DarkGray));
        frame.render_stateful_widget(
            scrollbar,
            area.inner(Margin {
                vertical: 1,
                horizontal: 0,
            }),
            &mut scrollbar_state,
        );
    }
}

fn turn_visible_rows(area: Rect) -> usize {
    usize::from(area.height.saturating_sub(3))
}

fn selected_turn_index(analytics: &SessionAnalytics, state: &ViewState) -> usize {
    analytics
        .turns
        .len()
        .checked_sub(1)
        .map_or(0, |last| state.selected.min(last))
}

fn visible_window_start(total_rows: usize, selected: usize, visible_rows: usize) -> usize {
    if total_rows == 0 || visible_rows == 0 || total_rows <= visible_rows {
        return 0;
    }
    let max_start = total_rows - visible_rows;
    selected.saturating_sub(visible_rows / 2).min(max_start)
}

fn render_aggregates(
    frame: &mut Frame,
    area: Rect,
    analytics: &SessionAnalytics,
    state: &ViewState,
) {
    let rows = activity_rows(analytics.turns.get(state.selected));
    let inner_height = area.height.saturating_sub(2);
    let visible_rows = usize::from(inner_height.saturating_sub(1));
    let (start, end) = activity_window(rows.len(), visible_rows, state.activity_scroll);
    let wide = area.width >= 86;
    let visible = rows[start..end]
        .iter()
        .map(|row| activity_table_row(row, wide));

    let table = Table::new(visible, activity_widths(wide))
        .header(activity_header(wide))
        .column_spacing(1)
        .block(Block::bordered().title(" Activity (source order) "));
    frame.render_widget(table, area);

    if visible_rows > 0 && rows.len() > visible_rows {
        let mut scrollbar_state = ScrollbarState::new(rows.len())
            .position(start)
            .viewport_content_length(visible_rows);
        let scrollbar = Scrollbar::new(ScrollbarOrientation::VerticalRight)
            .begin_symbol(Some("↑"))
            .end_symbol(Some("↓"))
            .thumb_style(Style::new().fg(Color::Cyan))
            .track_style(Style::new().fg(Color::DarkGray));
        frame.render_stateful_widget(
            scrollbar,
            area.inner(Margin {
                vertical: 1,
                horizontal: 0,
            }),
            &mut scrollbar_state,
        );
    }
}

#[derive(Debug, Clone)]
enum ActivityRow {
    Event {
        seq: String,
        kind: String,
        count: String,
        activity: String,
        /// AIC credit value for the enclosing turn (same for all activities in a turn).
        credits: String,
    },
}

fn activity_rows(turn: Option<&TurnMetrics>) -> Vec<ActivityRow> {
    let Some(turn) = turn else {
        return vec![ActivityRow::Event {
            seq: "-".to_string(),
            kind: "status".to_string(),
            count: "-".to_string(),
            activity: "no selected turn".to_string(),
            credits: "-".to_string(),
        }];
    };

    if turn.activity_events.is_empty() {
        return vec![ActivityRow::Event {
            seq: "-".to_string(),
            kind: "activity".to_string(),
            count: "0".to_string(),
            activity: "none recorded".to_string(),
            credits: turn.credit_display(),
        }];
    }

    grouped_activity_rows(turn)
}

fn grouped_activity_rows(turn: &TurnMetrics) -> Vec<ActivityRow> {
    let total = turn.activity_events.len();

    // First pass: group consecutive same-kind+name events so we know each
    // group's final count before computing its proportional credit share.
    let mut groups: Vec<(String, String, String, usize)> = Vec::new(); // (seq, kind, name, count)
    for (index, event) in turn.activity_events.iter().enumerate() {
        match groups.last_mut() {
            Some((_, kind, name, count)) if kind == &event.kind && name == &event.name => {
                *count += 1;
            }
            _ => groups.push((
                (index + 1).to_string(),
                event.kind.clone(),
                event.name.clone(),
                1,
            )),
        }
    }

    // Second pass: build rows with the credit share computed from the final count.
    groups
        .into_iter()
        .map(|(seq, kind, activity, count)| ActivityRow::Event {
            seq,
            kind: kind.clone(),
            count: count.to_string(),
            credits: activity_share_display(count, total, turn),
            activity,
        })
        .collect()
}

/// Proportional credit estimate for a single activity group.
///
/// Assumes each activity event in the turn consumes an equal share of the
/// total credits — the best available heuristic when per-event data is absent.
/// The `~` prefix signals that this is an estimate, not a measured value.
fn activity_share_display(count: usize, total: usize, turn: &TurnMetrics) -> String {
    if total == 0 {
        return "-".to_string();
    }
    let pct = (count as f64 / total as f64 * 100.0).round() as u32;
    match turn.credit_value() {
        Some(total_credits) => {
            let est = total_credits * count as f64 / total as f64;
            format!("~{pct}% {est:.1}")
        }
        None => format!("~{pct}%"),
    }
}

fn activity_table_row(row: &ActivityRow, wide: bool) -> Row<'static> {
    match row {
        ActivityRow::Event {
            seq,
            kind,
            count,
            activity,
            credits,
        } if wide => Row::new(vec![
            Cell::from(seq.clone()),
            Cell::from(kind.clone()),
            Cell::from(count.clone()),
            Cell::from(activity.clone()),
            Cell::from(credits.clone()),
        ])
        .style(activity_style(kind)),
        ActivityRow::Event {
            seq,
            kind,
            count,
            activity,
            ..
        } => Row::new(vec![
            Cell::from(seq.clone()),
            Cell::from(kind.clone()),
            Cell::from(count.clone()),
            Cell::from(activity.clone()),
        ])
        .style(activity_style(kind)),
    }
}

fn activity_header(wide: bool) -> Row<'static> {
    let cells = if wide {
        vec!["#", "kind", "cnt", "activity", "~share"]
    } else {
        vec!["#", "kind", "cnt", "activity"]
    };
    Row::new(cells).style(Style::new().add_modifier(Modifier::BOLD))
}

fn activity_widths(wide: bool) -> Vec<Constraint> {
    if wide {
        vec![
            Constraint::Length(3),
            Constraint::Length(7),
            Constraint::Length(3),
            Constraint::Min(18),
            Constraint::Length(12),
        ]
    } else {
        vec![
            Constraint::Length(3),
            Constraint::Length(5),
            Constraint::Length(3),
            Constraint::Min(12),
        ]
    }
}

fn activity_style(kind: &str) -> Style {
    match kind {
        "tool" => Style::new().fg(Color::Cyan),
        "cmd" => Style::new().fg(Color::Yellow),
        "skill" => Style::new().fg(Color::Magenta),
        "agent" => Style::new().fg(Color::Green),
        _ => Style::new().fg(Color::Gray),
    }
}

fn activity_window(
    total_rows: usize,
    visible_rows: usize,
    requested_scroll: usize,
) -> (usize, usize) {
    if total_rows == 0 || visible_rows == 0 {
        return (0, 0);
    }
    let max_start = total_rows.saturating_sub(visible_rows);
    let start = requested_scroll.min(max_start);
    let end = (start + visible_rows).min(total_rows);
    (start, end)
}

fn turn_palette_color(index: usize) -> Color {
    const PALETTE: [Color; 8] = [
        Color::Cyan,
        Color::Yellow,
        Color::Green,
        Color::Magenta,
        Color::Blue,
        Color::LightCyan,
        Color::LightYellow,
        Color::LightGreen,
    ];

    PALETTE[index % PALETTE.len()]
}

fn turn_row_style(index: usize, selected: bool) -> Style {
    let color = turn_palette_color(index);
    if selected {
        Style::new()
            .fg(Color::Black)
            .bg(color)
            .add_modifier(Modifier::BOLD)
    } else {
        Style::new().fg(color)
    }
}

fn timeline_segment_style(index: usize, selected: bool) -> Style {
    let color = turn_palette_color(index);
    if selected {
        Style::new()
            .bg(color)
            .fg(Color::Black)
            .add_modifier(Modifier::BOLD)
    } else {
        Style::new().bg(color).fg(Color::Black)
    }
}

fn render_footer(frame: &mut Frame, area: Rect, analytics: &SessionAnalytics, state: &ViewState) {
    let detail = match analytics.turns.get(state.selected) {
        Some(turn) => turn_detail(turn),
        None => "no turns".to_string(),
    };
    let lines = vec![
        Line::from(Span::styled(detail, Style::new().fg(Color::Gray))),
        Line::from(Span::styled(
            "↑/↓ select turn   activity PgUp/PgDn or [/]   q quit",
            Style::new().fg(Color::DarkGray),
        )),
    ];
    frame.render_widget(Paragraph::new(lines).block(Block::bordered()), area);
}

fn turn_detail(turn: &TurnMetrics) -> String {
    let mut parts = vec![
        format!("turn {}", turn.index),
        format!("{} in tok", token_value(turn.input_tokens)),
        format!("{} out tok", turn.output_tokens),
        format!("{} AIC", turn.credit_display()),
        format!("{:.1}% tok", turn.pct_output_tokens),
        format!("{:.0}s", turn.elapsed_ms.unwrap_or(0) as f64 / 1000.0),
        format!("{} tools", turn.tools.len()),
    ];
    if !turn.skills.is_empty() {
        parts.push(format!("skills: {}", dedup_join(&turn.skills)));
    }
    if !turn.subagents.is_empty() {
        parts.push(format!("subagents: {}", dedup_join(&turn.subagents)));
    }
    parts.join("   ")
}

/// Render a proportional bar of block glyphs, scaled to `max`.
fn bar_string(value: u64, max: u64, width: usize) -> String {
    if max == 0 || width == 0 {
        return String::new();
    }
    let filled = ((value as f64 / max as f64) * width as f64).round() as usize;
    "█".repeat(filled.min(width))
}

fn short_id(request_id: Option<&str>) -> String {
    let Some(id) = request_id else {
        return "-".to_string();
    };
    let trimmed = id.strip_prefix("request_").unwrap_or(id);
    trimmed.chars().take(12).collect()
}

fn dedup_join(values: &[String]) -> String {
    unique_values(values).join(", ")
}

fn unique_values(values: &[String]) -> Vec<String> {
    let mut seen = Vec::new();
    for value in values {
        if !seen.contains(value) {
            seen.push(value.clone());
        }
    }
    seen
}

/// Draw the bottom timeline: each turn is a section of one horizontal bar,
/// sized in proportion to its elapsed time, with the selected turn highlighted.
fn render_timeline(frame: &mut Frame, area: Rect, analytics: &SessionAnalytics, state: &ViewState) {
    let title = if analytics.turns.is_empty() {
        " Timeline ".to_string()
    } else {
        format!(
            " Timeline (width \u{221d} time) \u{2014} turn {} ",
            state.selected
        )
    };
    let block = Block::bordered().title(title);
    let inner = block.inner(area);
    frame.render_widget(block, area);

    if analytics.turns.is_empty() || inner.width == 0 {
        return;
    }

    let weights: Vec<u64> = analytics
        .turns
        .iter()
        .map(|turn| turn.elapsed_ms.unwrap_or(0).max(0) as u64)
        .collect();
    let lengths = timeline_segments(&weights, inner.width as usize);

    let mut spans = Vec::new();
    for (index, &length) in lengths.iter().enumerate() {
        if length == 0 {
            continue;
        }
        let selected = index == state.selected;
        let style = timeline_segment_style(index, selected);
        spans.push(Span::styled(segment_label(index, length, selected), style));
    }
    frame.render_widget(Paragraph::new(Line::from(spans)), inner);
}

/// Label a timeline section with its turn index when there is room, else pad it.
fn segment_label(index: usize, length: usize, selected: bool) -> String {
    if selected {
        return selected_segment_label(index, length);
    }

    let text = index.to_string();
    if length >= text.len() {
        let pad = length - text.len();
        let left = pad / 2;
        let right = pad - left;
        format!("{}{}{}", " ".repeat(left), text, " ".repeat(right))
    } else {
        " ".repeat(length)
    }
}

fn selected_segment_label(index: usize, length: usize) -> String {
    match length {
        0 => String::new(),
        1 => "█".to_string(),
        2 => "◀▶".to_string(),
        _ => {
            let text = index.to_string();
            let inner_width = length.saturating_sub(2);
            if inner_width == 0 {
                return "◀▶".chars().take(length).collect();
            }

            let inner = if inner_width >= text.len() {
                let pad = inner_width - text.len();
                let left = pad / 2;
                let right = pad - left;
                format!("{}{}{}", " ".repeat(left), text, " ".repeat(right))
            } else {
                text.chars().take(inner_width).collect()
            };

            format!("◀{}▶", inner)
        }
    }
}

/// Allocate `width` cells across turns in proportion to `weights`.
///
/// When `width >= weights.len()` every turn gets at least one cell so it stays
/// visible; the remaining cells are distributed by weight. The returned lengths
/// always sum to `width`.
fn timeline_segments(weights: &[u64], width: usize) -> Vec<usize> {
    let n = weights.len();
    if n == 0 || width == 0 {
        return vec![0; n];
    }
    if width <= n {
        return largest_remainder(weights, width);
    }
    let mut lengths = vec![1usize; n];
    for (slot, extra) in lengths
        .iter_mut()
        .zip(largest_remainder(weights, width - n))
    {
        *slot += extra;
    }
    lengths
}

/// Distribute `total` cells across `weights` using the largest-remainder method.
/// Falls back to an even split when every weight is zero.
fn largest_remainder(weights: &[u64], total: usize) -> Vec<usize> {
    let n = weights.len();
    if n == 0 || total == 0 {
        return vec![0; n];
    }
    let sum: u64 = weights.iter().sum();
    if sum == 0 {
        let base = total / n;
        let remainder = total % n;
        return (0..n).map(|i| base + usize::from(i < remainder)).collect();
    }

    let mut lengths = vec![0usize; n];
    let mut fractions: Vec<(f64, usize)> = Vec::with_capacity(n);
    let mut used = 0usize;
    for (i, &weight) in weights.iter().enumerate() {
        let exact = weight as f64 / sum as f64 * total as f64;
        let base = exact.floor() as usize;
        lengths[i] = base;
        used += base;
        fractions.push((exact - base as f64, i));
    }

    let mut remaining = total.saturating_sub(used);
    fractions.sort_by(|a, b| b.0.partial_cmp(&a.0).unwrap_or(std::cmp::Ordering::Equal));
    for (_, index) in fractions {
        if remaining == 0 {
            break;
        }
        lengths[index] += 1;
        remaining -= 1;
    }
    lengths
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn bar_scales_to_max() {
        assert_eq!(bar_string(0, 100, 10), "");
        assert_eq!(bar_string(100, 100, 10).chars().count(), 10);
        assert_eq!(bar_string(50, 100, 10).chars().count(), 5);
        assert_eq!(bar_string(10, 0, 10), "");
    }

    #[test]
    fn dedup_join_removes_repeats() {
        let values = vec!["a".to_string(), "a".to_string(), "b".to_string()];
        assert_eq!(dedup_join(&values), "a, b");
    }

    #[test]
    fn activity_style_colors_by_kind() {
        assert_eq!(activity_style("tool"), Style::new().fg(Color::Cyan));
        assert_eq!(activity_style("cmd"), Style::new().fg(Color::Yellow));
        assert_eq!(activity_style("skill"), Style::new().fg(Color::Magenta));
        assert_eq!(activity_style("agent"), Style::new().fg(Color::Green));
        assert_eq!(activity_style("other"), Style::new().fg(Color::Gray));
    }

    #[test]
    fn activity_rows_group_only_consecutive_matches() {
        let mut turn = TurnMetrics {
            index: 0,
            request_id: None,
            model: None,
            mode: None,
            timestamp_ms: None,
            output_tokens: 10,
            input_tokens: None,
            cached_tokens: None,
            elapsed_ms: None,
            first_progress_ms: None,
            output_credits: 0.0,
            credits: None,
            pct_output_tokens: 0.0,
            pct_time: 0.0,
            tools: Vec::new(),
            subagents: Vec::new(),
            skills: Vec::new(),
            terminal_commands: Vec::new(),
            activity_events: Vec::new(),
            had_reasoning: false,
        };
        turn.activity_events = vec![
            activity_event("tool", "read"),
            activity_event("tool", "read"),
            activity_event("skill", "plan"),
            activity_event("tool", "read"),
        ];

        let rows = activity_rows(Some(&turn));
        let summaries: Vec<(&str, &str, &str, &str)> = rows
            .iter()
            .map(|row| match row {
                ActivityRow::Event {
                    seq,
                    kind,
                    count,
                    activity,
                    ..
                } => (
                    seq.as_str(),
                    kind.as_str(),
                    count.as_str(),
                    activity.as_str(),
                ),
            })
            .collect();

        assert_eq!(
            summaries,
            vec![
                ("1", "tool", "2", "read"),
                ("3", "skill", "1", "plan"),
                ("4", "tool", "1", "read"),
            ]
        );
    }

    #[test]
    fn visible_window_keeps_selected_turn_centered_when_possible() {
        assert_eq!(visible_window_start(40, 25, 5), 23);
        assert_eq!(visible_window_start(40, 1, 5), 0);
        assert_eq!(visible_window_start(40, 39, 5), 35);
        assert_eq!(visible_window_start(3, 2, 5), 0);
        assert_eq!(visible_window_start(40, 25, 0), 0);
    }

    #[test]
    fn progress_label_distinguishes_empty_finished_scan() {
        let state = BrowserState::new(ScanProgress {
            processed: 0,
            total: 0,
            finished: true,
        });

        assert_eq!(progress_label(&state), "No sessions found   errors 0");
    }

    #[test]
    fn turn_palette_uses_multiple_distinct_colors() {
        let colors: Vec<Color> = (0..8).map(turn_palette_color).collect();
        let unique = colors.iter().fold(Vec::new(), |mut seen, color| {
            if !seen.contains(color) {
                seen.push(*color);
            }
            seen
        });

        assert!(unique.len() >= 6, "expected richer palette, got {unique:?}");
        assert_eq!(turn_palette_color(0), turn_palette_color(8));
    }

    #[test]
    fn turn_styles_share_palette_color() {
        let color = turn_palette_color(3);
        let normal = turn_row_style(3, false);
        let selected = turn_row_style(3, true);
        let timeline = timeline_segment_style(3, false);
        let selected_timeline = timeline_segment_style(3, true);

        assert_eq!(normal.fg, Some(color));
        assert_eq!(selected.bg, Some(color));
        assert_eq!(timeline.bg, Some(color));
        assert_eq!(selected_timeline.bg, Some(color));
    }

    #[test]
    fn selected_segment_label_uses_explicit_markers() {
        assert_eq!(segment_label(3, 1, true), "█");
        assert_eq!(segment_label(3, 2, true), "◀▶");
        assert_eq!(segment_label(3, 3, true), "◀3▶");
        assert_eq!(segment_label(3, 5, true), "◀ 3 ▶");
    }

    #[test]
    fn unselected_segment_label_stays_plain() {
        assert_eq!(segment_label(3, 1, false), "3");
        assert_eq!(segment_label(3, 3, false), " 3 ");
    }

    fn activity_event(kind: &str, name: &str) -> crate::analytics::ActivityEvent {
        crate::analytics::ActivityEvent {
            kind: kind.to_string(),
            name: name.to_string(),
        }
    }

    #[test]
    fn timeline_segments_sum_to_width() {
        let weights = [100, 300, 600];
        let lengths = timeline_segments(&weights, 20);
        assert_eq!(lengths.iter().sum::<usize>(), 20);
        // Heaviest turn gets the widest section.
        assert!(lengths[2] > lengths[1] && lengths[1] > lengths[0]);
    }

    #[test]
    fn timeline_segments_keep_every_turn_visible() {
        // width >= turn count: each turn gets at least one cell.
        let weights = [0, 0, 1000];
        let lengths = timeline_segments(&weights, 10);
        assert_eq!(lengths.iter().sum::<usize>(), 10);
        assert!(lengths.iter().all(|&l| l >= 1));
    }

    #[test]
    fn timeline_segments_handle_narrow_and_empty() {
        // Narrower than the turn count still sums to width.
        let weights = [1, 1, 1, 1, 1];
        assert_eq!(timeline_segments(&weights, 3).iter().sum::<usize>(), 3);
        // Empty input or zero width yields no cells.
        assert_eq!(timeline_segments(&[], 10), Vec::<usize>::new());
        assert_eq!(timeline_segments(&weights, 0), vec![0; 5]);
    }

    #[test]
    fn timeline_segments_equal_when_no_timing() {
        // All-zero weights fall back to an even split.
        let lengths = timeline_segments(&[0, 0, 0, 0], 8);
        assert_eq!(lengths, vec![2, 2, 2, 2]);
    }

    #[test]
    fn segment_label_centers_index_when_room() {
        assert_eq!(segment_label(3, 1, false), "3");
        assert_eq!(segment_label(3, 3, false), " 3 ");
        // No room for the digits: blank padding only.
        assert_eq!(segment_label(12, 1, false), " ");
    }
}
