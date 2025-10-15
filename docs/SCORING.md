# Cracked Dev‑o‑Meter: Scoring Specification

This document formalizes the scoring algorithm for the Cracked Dev‑o‑Meter. It defines postulates, features, transforms, aggregation, anti‑gaming rules, and implementation details to produce calibrated, explainable 0–100 scores from multi‑source developer signals (GitHub, X, etc.).

## 1) Postulates

- P1 Recency Monotonicity: The value of an event decays exponentially with age.
  - w(t) = exp(-(T - t)/tau), with short/long horizons tau_s in [30,90], tau_l in [180,365] days.
- P2 Diminishing Returns: Repeated similar events have concave utility.
  - Use concave transforms: asinh(x) or log(1+x).
- P3 Quality-over-Quantity: Peer validation dominates raw counts.
  - E.g., merged PRs with review depth >> raw commits; star velocity >> absolute stars.
- P4 Difficulty/Complexity Reward: Harder work (scope, review rounds, multi-language span) amplifies signal.
- P5 Collaboration Balance: Authorship and meaningful reviews both add value; trivial reviews discounted.
- P6 Reliability: CI pass rates and revert rarity boost trust; flaky behavior penalized.
- P7 Influence & Reach: Network centrality and downstream usage matter; velocity > stock.
- P8 Robustness to Gaming: Cap per-feature influence; prefer robust normalization; detect spam patterns.
- P9 Multi‑modal Synergy: Independent modalities combine multiplicatively in odds-space (Bayesian aggregation).
- P10 Explainability: Score decomposes into per-feature contributions with confidence.

## 2) Signals and Feature Families

- Shipping/Velocity: commits/day (decayed), merged PRs, PR lead time, issue turnaround
- Quality/Review: reviews received/given (depth), CI pass ratio, revert rate
- Influence: star velocity, fork velocity, dependent repos, follower centrality
- Complexity: files changed per PR, cross-language entropy, tests/docs presence
- Collaboration: PR-to-review ratio, unique collaborators, org diversity
- Reliability: release cadence regularity, bug re-open rates, flaky CI ratio
- Novelty/Learning: new language adoption, topic entropy, new‑repo velocity
- Social (X): technical-post engagement velocity, centrality (weighted modestly)

Each raw event e has timestamp t_e and attributes. Aggregate into per-feature values using event weights.

## 3) Transforms and Normalization

- Event decay: w_s(t)=exp(-(T-t)/tau_s), w_l(t)=exp(-(T-t)/tau_l)
- Dual-horizon blend: X* = lambda*X*{tau_s} + (1-lambda)\*X*{tau_l}, lambda in [0.6,0.8]
- Robust z-score (heavy-tail safe):
  - z = asinh( (X - median(X)) / (1.4826\*MAD(X)) )
  - Clip: z <- clip(z, -z_max, z_max), e.g. z_max=3

## 4) Category Subscores (Log‑odds)

Treat each category k as evidence for “cracked”:

ell*k = b_k + sum_j alpha*{k,j} \* z\_{k,j}

Apply quality gates (multipliers) where applicable: ell_k <- ell_k \* q_k, q_k in [0,1]

Recommended initial category weights (can be tuned):

- Shipping 0.25, Quality 0.20, Influence 0.20, Complexity 0.15, Collaboration 0.10, Reliability 0.07, Novelty 0.03

## 5) Bayesian Aggregation

Assuming first-order independence (approximation):

- Total log-odds: L = b_0 + sum_k w_k \* ell_k, sum_k w_k = 1
- Posterior crackedness: p = sigmoid(L) = 1/(1 + exp(-L))
- Final score: S = round(100 \* p)

Confidence: coverage factor c in [0,1] from data completeness; expose per-feature contributions.

## 6) Anti‑Gaming Rules

- Collapse near-duplicate commits/PRs (min spacing)
- Discount trivial changes and boilerplate reviews
- Penalize anomalous timing patterns (unless justified by collaborators/timezones)
- Cap per-feature contributions and apply robust normalization
- Exclude bot accounts and mirrors; normalize by peer group/domain

## 7) Influence via Growth, not Stock

- Star velocity via decayed star events
- Fork velocity similarly; dependent repos from ecosystem APIs
- Social/network centrality with decay

## 8) Complexity Proxies (no code checkout)

- Median/percentile files per PR
- Language/topic entropy: H = -sum p_l \* log p_l
- Tests/docs presence ratios (bounded contribution)
- Review rounds as difficulty amplifier

## 9) Calibration

- Robust baselines by domain (language/topic) using median and MAD
- Optional entropy weighting for alpha\_{k,j} (features with more dispersion have higher weight)
- Keep a bootstrap calibration set; fall back to defaults if unavailable

## 10) Output Schema

```
{
  "score": 83,
  "confidence": 0.78,
  "posterior": 0.83,
  "contributors": [
    {"name":"shipping.star_velocity","contribution":0.42},
    {"name":"quality.review_depth","contribution":0.31},
    {"name":"complexity.lang_entropy","contribution":0.18}
  ],
  "breakdown": {
    "shipping": 0.95,
    "quality": 0.76,
    "influence": 0.81,
    "complexity": 0.64,
    "collaboration": 0.58,
    "reliability": 0.61,
    "novelty": 0.43
  }
}
```

## 11) Implementation Plan (Backend)

- Ports & Adapters (Hexagonal): define interfaces and adapters per source (GitHub, X)
- Analysis Core: decay, robust normalization, feature builders, category scorers, aggregator
- Calibration: default baselines and per-domain overrides
- API: `/analyze` endpoint returns score + attribution + confidence
- Tests: unit tests for decay, robust stats, and aggregation

## 12) Defaults & Constants

- Horizons: tau_s = 60 days, tau_l = 270 days
- Blend: lambda = 0.7
- Clip: z_max = 3
- Weights w_k: Shipping 0.25, Quality 0.20, Influence 0.20, Complexity 0.15, Collaboration 0.10, Reliability 0.07, Novelty 0.03

---

This spec is designed for speed, robustness, and explainability. It is intentionally modular: new features can be added without destabilizing the aggregation or violating the postulates.
