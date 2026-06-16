//! Model pricing expressed in GitHub AI credits (AICs) per 1,000,000 tokens.

use serde::Serialize;

/// AIC rates for a model, expressed per 1,000,000 tokens.
#[derive(Debug, Clone, Copy, PartialEq, Serialize)]
pub struct ModelRates {
    pub input_per_m: f64,
    pub output_per_m: f64,
    pub cache_per_m: f64,
}

impl ModelRates {
    pub const fn new(input_per_m: f64, output_per_m: f64, cache_per_m: f64) -> Self {
        Self {
            input_per_m,
            output_per_m,
            cache_per_m,
        }
    }

    /// Credits for the given token counts. Each component contributes
    /// `tokens / 1_000_000 * rate`.
    pub fn credits(&self, input: u64, output: u64, cached: u64) -> f64 {
        (input as f64) / 1_000_000.0 * self.input_per_m
            + (output as f64) / 1_000_000.0 * self.output_per_m
            + (cached as f64) / 1_000_000.0 * self.cache_per_m
    }

    /// Credits attributable to output tokens only.
    pub fn output_credits(&self, output: u64) -> f64 {
        (output as f64) / 1_000_000.0 * self.output_per_m
    }
}

/// Normalize a model identifier for table lookups.
pub fn normalize_model_key(model: &str) -> String {
    model.trim().to_lowercase().replace(' ', "-")
}

/// Built-in fallback rates for known models, in AICs per 1M tokens.
///
/// Used only when a session log does not embed its own pricing.
pub fn builtin_rates(model: &str) -> Option<ModelRates> {
    match normalize_model_key(model).as_str() {
        "gpt-5.5" => Some(ModelRates::new(500.0, 3000.0, 50.0)),
        _ => None,
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn output_credits_match_rate() {
        let rates = ModelRates::new(500.0, 3000.0, 50.0);
        // 1,000,000 output tokens at 3000 AIC/1M == 3000 AIC.
        assert_eq!(rates.output_credits(1_000_000), 3000.0);
        // 4000 output tokens == 12 AIC.
        assert!((rates.output_credits(4000) - 12.0).abs() < 1e-9);
    }

    #[test]
    fn full_credits_sum_components() {
        let rates = ModelRates::new(500.0, 3000.0, 50.0);
        // 1M input + 1M output + 1M cached == 500 + 3000 + 50.
        assert!((rates.credits(1_000_000, 1_000_000, 1_000_000) - 3550.0).abs() < 1e-9);
    }

    #[test]
    fn builtin_lookup_is_case_insensitive() {
        assert_eq!(
            builtin_rates("GPT-5.5"),
            Some(ModelRates::new(500.0, 3000.0, 50.0))
        );
        assert_eq!(builtin_rates("unknown-model"), None);
    }
}
