//! Interactive terminal dashboard for [`SessionAnalytics`], built on ratatui.
//!
//! [`run`] drives a full-screen interactive view. [`render`] is a pure drawing
//! function (no terminal side effects) so it can be exercised with ratatui's
//! `TestBackend` in unit tests.

use std::io::{self, Stdout};

use anyhow::Result;
use ratatui::backend::CrosstermBackend;
use ratatui::crossterm::{
    event::{self, Event, KeyCode, KeyEventKind},
    execute,
    terminal::{disable_raw_mode, enable_raw_mode, EnterAlternateScreen, LeaveAlternateScreen},
};
use ratatui::layout::{Constraint, Layout, Margin, Rect};
use ratatui::style::{Color, Modifier, Style, Stylize};
use ratatui::text::{Line, Span};
use ratatui::widgets::{
    Block, Cell, Paragraph, Row, Scrollbar, ScrollbarOrientation, ScrollbarState, Table,
};
use ratatui::{Frame, Terminal};

use crate::analytics::{CreditSource, RatesSource, SessionAnalytics, TurnMetrics};

/// View state shared between the event loop and [`render`].
#[derive(Debug, Default, Clone, Copy)]
pub struct ViewState {
    /// Index of the currently highlighted turn.
    pub selected: usize,
    /// First visible row in the activity usage table.
    pub activity_scroll: usize,
}

/// Launch the interactive dashboard, returning when the user quits.
pub fn run(analytics: &SessionAnalytics) -> Result<()> {
    let mut terminal = setup_terminal()?;
    let result = event_loop(&mut terminal, analytics);
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

    let rows = analytics.turns.iter().map(|turn| {
        let bar = bar_string(turn.output_tokens, max_tokens, 14);
        let selected = turn.index == state.selected;
        let style = if selected {
            Style::new().fg(Color::Black).bg(Color::Cyan)
        } else {
            Style::new()
        };
        Row::new(vec![
            Cell::from(turn.index.to_string()),
            Cell::from(short_id(turn.request_id.as_deref())),
            Cell::from(token_value(turn.input_tokens)),
            Cell::from(turn.output_tokens.to_string()),
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
        Constraint::Min(14),
    ];
    let header = Row::new(vec![
        "#",
        "request",
        "in_tok",
        "out_tok",
        "%tok",
        "output share",
    ])
    .style(Style::new().add_modifier(Modifier::BOLD));
    let table = Table::new(rows, widths)
        .header(header)
        .block(Block::bordered().title(" Turns "));
    frame.render_widget(table, area);
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
        .block(Block::bordered().title(" Activity "));
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
    Section(&'static str),
    Detail { field: String, value: String },
}

fn activity_rows(turn: Option<&TurnMetrics>) -> Vec<ActivityRow> {
    let Some(turn) = turn else {
        return vec![ActivityRow::Detail {
            field: "status".to_string(),
            value: "no selected turn".to_string(),
        }];
    };

    let mut rows = vec![ActivityRow::Section("Turn")];
    push_detail(&mut rows, "index", format!("turn {}", turn.index));
    push_detail(&mut rows, "request", short_id(turn.request_id.as_deref()));
    if let Some(model) = &turn.model {
        push_detail(&mut rows, "model", model.clone());
    }
    if let Some(mode) = &turn.mode {
        push_detail(&mut rows, "mode", mode.clone());
    }
    push_detail(
        &mut rows,
        "input",
        format!("{} tok", token_value(turn.input_tokens)),
    );
    push_detail(&mut rows, "output", format!("{} tok", turn.output_tokens));
    if let Some(cached) = turn.cached_tokens {
        push_detail(&mut rows, "cached", format!("{cached} tok"));
    }
    push_detail(
        &mut rows,
        "output %",
        format!("{:.1}%", turn.pct_output_tokens),
    );
    push_detail(&mut rows, "credits", turn_credit_text(turn));
    push_detail(
        &mut rows,
        "elapsed",
        format!("{:.1}s", turn.elapsed_ms.unwrap_or(0) as f64 / 1000.0),
    );

    push_named_values(&mut rows, "Tools", &turn.tools);
    push_named_values(&mut rows, "Commands", &turn.terminal_commands);
    push_named_values(&mut rows, "Skills", &turn.skills);
    push_named_values(&mut rows, "Subagents", &turn.subagents);
    rows
}

fn push_detail(rows: &mut Vec<ActivityRow>, field: &str, value: String) {
    rows.push(ActivityRow::Detail {
        field: field.to_string(),
        value,
    });
}

fn push_named_values(rows: &mut Vec<ActivityRow>, title: &'static str, values: &[String]) {
    if values.is_empty() {
        return;
    }
    rows.push(ActivityRow::Section(title));
    for (index, value) in unique_values(values).into_iter().enumerate() {
        push_detail(rows, &(index + 1).to_string(), value);
    }
}

fn activity_table_row(row: &ActivityRow, wide: bool) -> Row<'static> {
    match row {
        ActivityRow::Section(title) => section_row(title, wide),
        ActivityRow::Detail { field, value } if wide => Row::new(vec![
            Cell::from(""),
            Cell::from(field.clone()),
            Cell::from(value.clone()),
        ]),
        ActivityRow::Detail { field, value } => {
            Row::new(vec![Cell::from(field.clone()), Cell::from(value.clone())])
        }
    }
}

fn section_row(title: &'static str, wide: bool) -> Row<'static> {
    let mut cells = vec![Cell::from(Span::styled(
        title.to_string(),
        Style::new().fg(Color::Magenta).add_modifier(Modifier::BOLD),
    ))];
    let column_count = if wide { 3 } else { 2 };
    cells.extend((1..column_count).map(|_| Cell::from("")));
    Row::new(cells)
}

fn activity_header(wide: bool) -> Row<'static> {
    let cells = if wide {
        vec!["section", "field", "value"]
    } else {
        vec!["field", "value"]
    };
    Row::new(cells).style(Style::new().add_modifier(Modifier::BOLD))
}

fn activity_widths(wide: bool) -> Vec<Constraint> {
    if wide {
        vec![
            Constraint::Length(10),
            Constraint::Length(12),
            Constraint::Min(20),
        ]
    } else {
        vec![Constraint::Length(9), Constraint::Min(12)]
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

fn turn_credit_text(turn: &TurnMetrics) -> String {
    match turn.credits {
        Some(total) => format!("{total:.2} rpt"),
        None if turn.output_credits > 0.0 => format!("{:.2} AIC output", turn.output_credits),
        None => "n/a".to_string(),
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
        let style = if index == state.selected {
            Style::new()
                .bg(Color::Cyan)
                .fg(Color::Black)
                .add_modifier(Modifier::BOLD)
        } else if index % 2 == 0 {
            Style::new().bg(Color::Blue).fg(Color::White)
        } else {
            Style::new().bg(Color::DarkGray).fg(Color::White)
        };
        spans.push(Span::styled(segment_label(index, length), style));
    }
    frame.render_widget(Paragraph::new(Line::from(spans)), inner);
}

/// Label a timeline section with its turn index when there is room, else pad it.
fn segment_label(index: usize, length: usize) -> String {
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
        assert_eq!(segment_label(3, 1), "3");
        assert_eq!(segment_label(3, 3), " 3 ");
        // No room for the digits: blank padding only.
        assert_eq!(segment_label(12, 1), " ");
    }
}
