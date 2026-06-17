//! Render the TUI to an in-memory backend and assert key content appears.

use ratatui::backend::TestBackend;
use ratatui::buffer::Buffer;
use ratatui::Terminal;

use vibe_watch::analytics::SessionAnalytics;
use vibe_watch::chat_log;
use vibe_watch::cli_log;
use vibe_watch::session_scan::{LoadedSession, ScanProgress, SessionLoadError};
use vibe_watch::tui::{render, render_browser, BrowserState, BrowserView, ViewState};

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

fn line_index(text: &str, needle: &str) -> usize {
    text.lines()
        .position(|line| line.contains(needle))
        .unwrap_or_else(|| panic!("missing {needle}:\n{text}"))
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

fn render_browser_to_text(state: &BrowserState, width: u16, height: u16) -> String {
    let backend = TestBackend::new(width, height);
    let mut terminal = Terminal::new(backend).expect("terminal");
    terminal
        .draw(|frame| render_browser(frame, state))
        .expect("draw");
    buffer_text(terminal.backend().buffer())
}

fn loaded_session(path: &str, analytics: SessionAnalytics) -> LoadedSession {
    LoadedSession {
        path: std::path::PathBuf::from(path),
        modified: std::time::SystemTime::UNIX_EPOCH,
        analytics,
    }
}

fn load_error(path: &str, message: &str) -> SessionLoadError {
    SessionLoadError {
        path: std::path::PathBuf::from(path),
        modified: None,
        message: message.to_string(),
    }
}

fn browser_state_with_sessions() -> BrowserState {
    let mut state = BrowserState::new(ScanProgress {
        processed: 2,
        total: 3,
        finished: false,
    });

    let mut vscode = load_chat_analytics();
    vscode.repository_path = Some("/tmp/vibe-watch-vscode".to_string());
    let mut cli = load_cli_analytics();
    cli.repository_path = Some("/tmp/vibe-watch-cli".to_string());
    state.add_loaded(loaded_session("/tmp/vscode/chat_min.jsonl", vscode));
    state.add_loaded(loaded_session("/tmp/cli/events.jsonl", cli));
    state.add_error(load_error(
        "/tmp/bad/events.jsonl",
        "unrecognized log format",
    ));
    state
}

fn activity_pane_text(text: &str) -> String {
    text.lines()
        .filter(|line| line.contains('│'))
        .filter_map(|line| line.rsplit_once('│').map(|(_, right)| right))
        .collect::<Vec<_>>()
        .join("\n")
}

#[test]
fn browser_dashboard_lists_repositories_and_session_counts() {
    let state = browser_state_with_sessions();
    let text = render_browser_to_text(&state, 140, 32);

    assert!(
        text.contains("Repositories"),
        "missing dashboard title:\n{text}"
    );
    assert!(
        text.contains("/tmp/vibe-watch-vscode"),
        "missing VS Code repo row:\n{text}"
    );
    assert!(
        text.contains("/tmp/vibe-watch-cli"),
        "missing CLI repo row:\n{text}"
    );
    assert!(
        text.contains("sessions"),
        "missing session count column:\n{text}"
    );
    assert!(text.contains("turns"), "missing turn count column:\n{text}");
    assert!(
        text.contains("12.0 reported"),
        "missing reported AIC:\n{text}"
    );
    assert!(text.contains("3.0 reported"), "missing CLI AIC:\n{text}");
}

#[test]
fn browser_repository_rows_include_selected_marker() {
    let mut state = browser_state_with_sessions();
    state.selected_repo = 1;
    let text = render_browser_to_text(&state, 140, 32);

    let selected_line = text
        .lines()
        .find(|line| line.contains("/tmp/vibe-watch-cli"))
        .unwrap_or_else(|| panic!("missing selected repo row:\n{text}"));
    assert!(
        selected_line.contains('>'),
        "selected repository row should include text marker:\n{text}"
    );

    let unselected_line = text
        .lines()
        .find(|line| line.contains("/tmp/vibe-watch-vscode"))
        .unwrap_or_else(|| panic!("missing unselected repo row:\n{text}"));
    assert!(
        !unselected_line.contains('>'),
        "unselected repository row should not include marker:\n{text}"
    );
}

#[test]
fn browser_session_list_shows_turns_aic_and_errors() {
    let mut state = browser_state_with_sessions();
    state.view = BrowserView::Sessions;
    state.selected_repo = 2;

    let text = render_browser_to_text(&state, 140, 32);

    assert!(
        text.contains("Sessions"),
        "missing session list title:\n{text}"
    );
    assert!(
        text.contains("bad/events.jsonl") || text.contains("events.jsonl"),
        "missing error row path:\n{text}"
    );
    assert!(
        text.contains("unrecognized log format"),
        "missing error message:\n{text}"
    );

    state.selected_repo = 0;
    let text = render_browser_to_text(&state, 140, 32);
    assert!(text.contains("test-session"), "missing session id:\n{text}");
    assert!(text.contains("2"), "missing turn count:\n{text}");
    assert!(
        text.contains("12.0 reported"),
        "missing session AIC:\n{text}"
    );
}

#[test]
fn browser_session_rows_include_selected_marker() {
    let mut state = browser_state_with_sessions();
    state.view = BrowserView::Sessions;
    state.selected_repo = 0;
    state.selected_session = 0;

    let text = render_browser_to_text(&state, 140, 32);
    let selected_line = text
        .lines()
        .find(|line| line.contains("test-session"))
        .unwrap_or_else(|| panic!("missing selected session row:\n{text}"));
    assert!(
        selected_line.contains('>'),
        "selected session row should include text marker:\n{text}"
    );
}

#[test]
fn browser_detail_reuses_selected_session_turn_activity_view() {
    let mut state = browser_state_with_sessions();
    state.view = BrowserView::SessionDetail;
    state.selected_repo = 0;
    state.selected_session = 0;
    state.detail.selected = 1;

    let text = render_browser_to_text(&state, 140, 32);

    assert!(
        text.contains("Turns"),
        "missing detail turns panel:\n{text}"
    );
    assert!(text.contains("Activity"), "missing activity panel:\n{text}");
    assert!(
        text.contains("turn 1"),
        "missing selected turn detail:\n{text}"
    );
    assert!(
        text.contains("run_in_terminal"),
        "missing selected session activity:\n{text}"
    );
}

#[test]
fn browser_progress_bar_updates_while_loading() {
    let state = browser_state_with_sessions();
    let text = render_browser_to_text(&state, 120, 24);

    assert!(text.contains("Loading"), "missing progress label:\n{text}");
    assert!(text.contains("2/3"), "missing processed/total:\n{text}");
    assert!(text.contains("errors 1"), "missing error count:\n{text}");
}

#[test]
fn browser_progress_reports_finished_empty_scan() {
    let state = BrowserState::new(ScanProgress {
        processed: 0,
        total: 0,
        finished: true,
    });

    let text = render_browser_to_text(&state, 120, 24);

    assert!(
        text.contains("No sessions found"),
        "missing empty scan progress label:\n{text}"
    );
    assert!(text.contains("errors 0"), "missing error count:\n{text}");
}

#[test]
fn browser_dashboard_uses_unknown_label_for_blank_repository_path() {
    let mut state = BrowserState::new(ScanProgress {
        processed: 1,
        total: 1,
        finished: true,
    });
    let mut analytics = load_chat_analytics();
    analytics.repository_path = Some("   ".to_string());
    state.add_loaded(loaded_session("/tmp/blank/chat.jsonl", analytics));

    let text = render_browser_to_text(&state, 120, 24);

    assert!(
        text.contains("(unknown repository)"),
        "blank repository should use fallback label:\n{text}"
    );
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
    let text = render_analytics_to_text(
        &analytics,
        140,
        30,
        &ViewState {
            selected: 1,
            ..ViewState::default()
        },
    );

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
    assert!(text.contains("AIC"), "missing AIC column:\n{text}");
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
    assert!(text.contains("4.5"), "missing turn 0 AIC:\n{text}");
    assert!(text.contains("7.5"), "missing turn 1 AIC:\n{text}");

    // Activity table follows the selected turn (turn 0 by default).
    assert!(text.contains("kind"), "missing kind column:\n{text}");
    assert!(text.contains("cnt"), "missing count column:\n{text}");
    assert!(
        text.contains("activity"),
        "missing activity column:\n{text}"
    );
    let activity = activity_pane_text(&text);
    assert!(
        !activity.contains("AIC"),
        "Activity pane should not show AIC:\n{text}"
    );
    assert!(
        text.contains("copilot_applyPatch"),
        "missing turn 0 tool:\n{text}"
    );
    assert!(
        !text.contains("run_in_terminal"),
        "turn 1 tool should not appear for selected turn 0:\n{text}"
    );
    assert!(
        !text.contains("demo-skill"),
        "turn 1 skill should not appear for selected turn 0:\n{text}"
    );
}

#[test]
fn renders_activity_for_selected_turn() {
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
        "missing selected turn label:\n{text}"
    );
    assert!(
        text.contains("run_in_terminal"),
        "missing selected turn tool:\n{text}"
    );
    assert!(
        text.contains("demo-skill"),
        "missing selected turn skill:\n{text}"
    );
    assert!(
        text.contains("cargo test"),
        "missing selected turn command:\n{text}"
    );
    let terminal_line = line_index(&text, "run_in_terminal");
    let command_line = line_index(&text, "cargo test");
    let read_file_line = line_index(&text, "copilot_readFile");
    let skill_line = line_index(&text, "demo-skill");
    assert!(
        terminal_line < command_line
            && command_line < read_file_line
            && read_file_line < skill_line,
        "activity rows should follow source order:\n{text}"
    );
    assert!(
        !activity_pane_text(&text).contains("7.50"),
        "Activity pane should not show selected turn credits:\n{text}"
    );
    assert!(
        !text.contains("copilot_applyPatch"),
        "turn 0 tool should not appear for selected turn 1:\n{text}"
    );
}

#[test]
fn renders_wide_selected_turn_activity_columns() {
    let text = render_to_text(240, 30, &ViewState::default());

    assert!(text.contains("kind"), "missing kind column:\n{text}");
    assert!(text.contains("cnt"), "missing count column:\n{text}");
    assert!(
        text.contains("activity"),
        "missing activity column:\n{text}"
    );
    assert!(text.contains("out"), "missing output token column:\n{text}");
    assert!(
        !activity_pane_text(&text).contains("assoc"),
        "Activity pane should not show associated AIC column:\n{text}"
    );
    assert!(
        text.contains("copilot_applyPatch"),
        "missing selected turn tool in wide mode:\n{text}"
    );
    assert!(
        !text.contains("demo-skill"),
        "unselected turn skill should not appear in wide mode:\n{text}"
    );
}

#[test]
fn renders_cli_subagent_activity_usage() {
    let analytics = load_cli_analytics();
    let text = render_analytics_to_text(
        &analytics,
        140,
        30,
        &ViewState {
            selected: 1,
            ..ViewState::default()
        },
    );

    assert!(text.contains("agent"), "missing subagent kind:\n{text}");
    assert!(text.contains("Explore"), "missing subagent name:\n{text}");
    assert!(
        text.contains("600"),
        "missing subagent output tokens:\n{text}"
    );
    assert!(
        !activity_pane_text(&text).contains("1.80"),
        "Activity pane should not show subagent output credits:\n{text}"
    );
}

#[test]
fn renders_scrolled_activity_rows() {
    let state = ViewState {
        selected: 0,
        activity_scroll: 999,
    };
    let text = render_to_text(120, 20, &state);

    assert!(text.contains("Activity"), "missing activity panel:\n{text}");
    assert!(
        text.contains("tool"),
        "missing selected turn tools section:\n{text}"
    );
    assert!(
        text.contains("copilot_applyPatch"),
        "missing selected turn tool after scroll clamp:\n{text}"
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
fn renders_long_turn_list_with_selected_turn_visible() {
    let mut analytics = load_chat_analytics();
    let template = analytics.turns[0].clone();
    analytics.turns = (0..40)
        .map(|index| {
            let mut turn = template.clone();
            turn.index = index;
            turn.request_id = Some(format!("request_long-{index:02}"));
            turn.output_tokens = (index as u64 + 1) * 10;
            turn.pct_output_tokens = index as f64;
            turn
        })
        .collect();
    analytics.turn_count = analytics.turns.len();

    let text = render_analytics_to_text(
        &analytics,
        120,
        20,
        &ViewState {
            selected: 25,
            ..ViewState::default()
        },
    );

    assert!(
        text.contains("long-25"),
        "selected turn should remain visible in long lists:\n{text}"
    );
    assert!(
        !text.contains("long-00"),
        "long lists should not always render from the first turn:\n{text}"
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
