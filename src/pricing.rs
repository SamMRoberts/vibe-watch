//! Model pricing expressed in GitHub AI credits (AICs) per 1,000,000 tokens.

use std::sync::OnceLock;

use serde::{Deserialize, Serialize};

/// Pricing table embedded at compile time from `config/model_pricing.json`.
///
/// Edit that file to update rates; no Rust changes are required.
const PRICING_JSON: &str = include_str!("../config/model_pricing.json");

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

/// One model's pricing entry as stored in `config/model_pricing.json`.
#[derive(Debug, Clone, Deserialize)]
struct ModelPricing {
    id: String,
    #[serde(default)]
    name: String,
    #[serde(default)]
    context_tokens: Option<u64>,
    input: f64,
    cache: f64,
    output: f64,
}

#[derive(Debug, Deserialize)]
struct PricingConfig {
    models: Vec<ModelPricing>,
}

/// The parsed pricing table, loaded once on first use.
fn pricing_table() -> &'static [ModelPricing] {
    static TABLE: OnceLock<Vec<ModelPricing>> = OnceLock::new();
    TABLE
        .get_or_init(|| {
            serde_json::from_str::<PricingConfig>(PRICING_JSON)
                .expect("embedded config/model_pricing.json must be valid JSON")
                .models
        })
        .as_slice()
}

/// The context window (in tokens) for a known model, if listed.
pub fn context_tokens(model: &str) -> Option<u64> {
    let key = normalize_model_key(model);
    pricing_table()
        .iter()
        .find(|entry| matches_model(entry, &key))
        .and_then(|entry| entry.context_tokens)
}

/// Built-in rates for known models, in AICs per 1M tokens.
///
/// Used when a session log reports token usage but not credits. Rates come from
/// the embedded `config/model_pricing.json` table, matched by `id` or `name`.
pub fn builtin_rates(model: &str) -> Option<ModelRates> {
    let key = normalize_model_key(model);
    pricing_table()
        .iter()
        .find(|entry| matches_model(entry, &key))
        .map(|entry| ModelRates::new(entry.input, entry.output, entry.cache))
}

fn matches_model(entry: &ModelPricing, normalized_key: &str) -> bool {
    normalize_model_key(&entry.id) == normalized_key
        || (!entry.name.is_empty() && normalize_model_key(&entry.name) == normalized_key)
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

    #[test]
    fn config_table_loads_all_models() {
        // Every entry in the embedded config should be resolvable.
        assert!(pricing_table().len() >= 17);
        for entry in pricing_table() {
            assert!(builtin_rates(&entry.id).is_some(), "missing {}", entry.id);
        }
    }

    #[test]
    fn lookup_matches_id_and_display_name() {
        // Claude Haiku 4.5: In 100, Cache 10, Out 500.
        let by_id = builtin_rates("claude-haiku-4.5").expect("by id");
        let by_name = builtin_rates("Claude Haiku 4.5").expect("by name");
        assert_eq!(by_id, by_name);
        assert_eq!(by_id, ModelRates::new(100.0, 500.0, 10.0));
    }

    #[test]
    fn context_tokens_are_available() {
        assert_eq!(context_tokens("gpt-5.5"), Some(1_000_000));
        assert_eq!(context_tokens("gpt-5-mini"), Some(192_000));
        assert_eq!(context_tokens("unknown"), None);
    }
}
