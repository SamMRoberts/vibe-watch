use std::process::ExitCode;

fn main() -> ExitCode {
    match vibe_watch::cli::run() {
        Ok(()) => ExitCode::SUCCESS,
        Err(err) => {
            eprintln!("error: {err:#}");
            ExitCode::FAILURE
        }
    }
}
