//! Render the TUI to an in-memory backend and assert key content appears.

use ratatui::backend::TestBackend;
use ratatui::buffer::Buffer;
use ratatui::Terminal;

use vibe_watch::analytics::SessionAnalytics;
use vibe_watch::chat_log;
use vibe_watch::cli_log;
use vibe_watch::tui::{render, ViewState};

fn load_chat_analytics() -> SessionAnalytics {
    let path = concat!(env!("CARGO_MANIFEST_DIR"), "/tests/fixtures/chat_min.jsonl");
    let data = std::fs::read_to_string(path).expect("read fixture");
    let session = chat_log::parse_str(&data).expect("parse fixture");
    let mut analytics = SessionAnalytics::from_chat(&session);
    analytics.repository_path = Some("/tmp/vibe-watch-vscode".to_string());
    analytics
}

fn load_cli_analytics() -> SessionAnalytics {
    let path = concat!(env!("CARGO_MANIFEST_DIR"), "/tests/fixtures/cli_min.jsonl");
    let data = std::fs::read_to_string(path).expect("read fixture");
    let session = cli_log::parse_str(&data).expect("parse fixture");
    let mut analytics = SessionAnalytics::from_cli(&session);
    analytics.repository_path = Some("/tmp/vibe-watch-cli".to_string());
    analytics
}

fn buffer_text(buffer: &Buffer) -> String {
    let area = buffer.area;
    let mut text = String::new();
    for y in 0..area.height {
        for x in 0..area.width {
            if let Some(cell) = buffer.cell((x, y)) {
                text.push_str(cell.symbol());
            }
        }
        text.push('\n');
    }
    text
}

fn render_analytics_to_text(
    analytics: &SessionAnalytics,
    width: u16,
    height: u16,
    state: &ViewState,
) -> String {
    let backend = TestBackend::new(width, height);
    let mut terminal = Terminal::new(backend).expect("terminal");
    terminal
        .draw(|frame| render(frame, analytics, state))
        .expect("draw");
    buffer_text(terminal.backend().buffer())
}

fn render_to_text(width: u16, height: u16, state: &ViewState) -> String {
    let analytics = load_chat_analytics();
    render_analytics_to_text(&analytics, width, height, state)
}

#[test]
fn renders_header_and_totals() {
    let text = render_to_text(120, 30, &ViewState::default());
    assert!(text.contains("vibe-watch"), "missing title:\n{text}");
    assert!(
        text.contains("/tmp/vibe-watch-vscode"),
        "missing repo path:\n{text}"
    );
    assert!(text.contains("GPT-5.5"), "missing model:\n{text}");
    assert!(
        text.contains("in 5000   out 4000   cached n/a"),
        "missing total tokens:\n{text}"
    );
    assert!(
        text.contains("12.0 AIC reported"),
        "missing reported credits:\n{text}"
    );
}

#[test]
fn renders_cli_token_categories_and_reported_credits() {
    let analytics = load_cli_analytics();
    let text = render_analytics_to_text(&analytics, 140, 30, &ViewState::default());

    assert!(
        text.contains("/tmp/vibe-watch-cli"),
        "missing repo path:\n{text}"
    );
    assert!(
        text.contains("in 10000   out 800   cached 7000"),
        "missing token categories:\n{text}"
    );
    assert!(
        text.contains("3.0 AIC reported"),
        "missing reported credits:\n{text}"
    );
}

#[test]
fn renders_turns_and_activity() {
    let text = render_to_text(120, 30, &ViewState::default());
    assert!(text.contains("Turns"), "missing turns panel:\n{text}");
    assert!(text.contains("Activity"), "missing activity panel:\n{text}");
    assert!(
        text.contains("in_tok"),
        "missing input token column:\n{text}"
    );
    assert!(
        text.contains("2000"),
        "missing turn 0 input tokens:\n{text}"
    );
    // Per-turn output token counts from the fixture.
    assert!(text.contains("1000"), "missing turn 0 tokens:\n{text}");
    assert!(text.contains("3000"), "missing turn 1 tokens:\n{text}");

    // Activity usage table.
    assert!(text.contains("use"), "missing calls/reqs column:\n{text}");
    assert!(text.contains("out"), "missing output token column:\n{text}");
    assert!(text.contains("AIC"), "missing credit column:\n{text}");
    assert!(text.contains("demo-skill"), "missing skill:\n{text}");
    assert!(text.contains("run_in_terminal"), "missing tool:\n{text}");
    assert!(
        text.contains("7.50 rpt"),
        "missing reported skill credits:\n{text}"
    );
}

#[test]
fn renders_wide_activity_usage_columns() {
    let text = render_to_text(240, 30, &ViewState::default());

    assert!(text.contains("calls"), "missing calls column:\n{text}");
    assert!(text.contains("reqs"), "missing reqs column:\n{text}");
    assert!(
        text.contains("outAIC"),
        "missing output credit column:\n{text}"
    );
    assert!(
        text.contains("9.00 out"),
        "missing output-only skill credits:\n{text}"
    );
}

#[test]
fn renders_cli_subagent_activity_usage() {
    let analytics = load_cli_analytics();
    let text = render_analytics_to_text(&analytics, 140, 30, &ViewState::default());

    assert!(
        text.contains("Subagents"),
        "missing subagent section:\n{text}"
    );
    assert!(text.contains("Explore"), "missing subagent name:\n{text}");
    assert!(
        text.contains("600"),
        "missing subagent output tokens:\n{text}"
    );
    assert!(
        text.contains("1.80 out"),
        "missing subagent output credits:\n{text}"
    );
}

#[test]
fn renders_scrolled_activity_rows() {
    let state = ViewState {
        selected: 0,
        activity_scroll: 4,
    };
    let text = render_to_text(120, 20, &state);

    assert!(text.contains("Activity"), "missing activity panel:\n{text}");
    assert!(
        text.contains("Skills"),
        "missing scrolled skill section:\n{text}"
    );
    assert!(
        text.contains("demo-skill"),
        "missing scrolled skill row:\n{text}"
    );
    assert!(
        text.contains("activity PgUp/PgDn"),
        "missing activity scroll help:\n{text}"
    );
}

#[test]
fn renders_selected_turn_input_token_detail() {
    let text = render_to_text(120, 30, &ViewState::default());
    assert!(
        text.contains("2000 in tok"),
        "missing selected turn input detail:\n{text}"
    );
}

#[test]
fn renders_without_panic_on_tiny_area() {
    // Should clip gracefully rather than panic on a small terminal.
    let _ = render_to_text(20, 8, &ViewState::default());
}

#[test]
fn renders_timeline_panel() {
    // Default selection (turn 0).
    let text = render_to_text(120, 30, &ViewState::default());
    assert!(text.contains("Timeline"), "missing timeline panel:\n{text}");
    assert!(
        text.contains("turn 0"),
        "timeline should note the selected turn:\n{text}"
    );

    // Selecting turn 1 updates the timeline title and still renders.
    let text = render_to_text(
        120,
        30,
        &ViewState {
            selected: 1,
            ..ViewState::default()
        },
    );
    assert!(
        text.contains("turn 1"),
        "timeline should follow selection:\n{text}"
    );
}
