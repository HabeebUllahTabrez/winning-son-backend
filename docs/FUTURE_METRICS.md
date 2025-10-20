# WinningSon-inator: Future Dashboard Metrics

**Last Updated:** 2025-10-15
**Status:** Roadmap for implementation

This document contains advanced metrics (Tier 2 and Tier 3) that will provide deeper insights into user patterns and behavior. These metrics are designed to help users "learn from themselves" and become "conscious architects of their lives."

---

## Table of Contents

- [Tier 2 Metrics - Medium Impact, Medium Effort](#tier-2-metrics---medium-impact-medium-effort)
- [Tier 3 Metrics - Advanced Features](#tier-3-metrics---advanced-features)
- [Implementation Guidelines](#implementation-guidelines)
- [Performance Considerations](#performance-considerations)
- [Data Requirements](#data-requirements)

---

## Tier 2 Metrics - Medium Impact, Medium Effort

### 1. Alignment-Contentment Correlation

**Description:** Calculate the correlation coefficient between alignment and contentment ratings to understand if they move together or independently.

**Value Proposition:** Reveals whether users feel good when they're aligned with goals (positive correlation) or if they sacrifice happiness for goal pursuit (negative correlation).

**SQL Implementation:**
```sql
SELECT
    CORR(alignment_rating, contentment_rating) AS alignment_contentment_correlation
FROM journal_entries
WHERE user_id = $1
  AND local_date >= $2 - INTERVAL '30 days'  -- Last 30 days for relevance
  AND local_date <= $2;
```

**Response Field:**
- `alignment_contentment_correlation` (float, -1 to 1)
  - `> 0.7`: Strong positive - happiness and alignment move together
  - `0.3 to 0.7`: Moderate positive
  - `-0.3 to 0.3`: Weak or no correlation - independent patterns
  - `< -0.3`: Negative - potential burnout indicator

**Performance:** O(n) single pass, very fast with index on `(user_id, local_date)`

---

### 2. Goal Countdown (with Deadline)

**Description:** Days remaining until goal deadline, only if user has set an end date.

**Value Proposition:** Creates urgency and helps users understand if they're on track.

**SQL Implementation:**
```sql
SELECT
    g.end_date,
    g.end_date - $2::date AS days_remaining,
    CASE
        WHEN g.end_date < $2::date THEN 'overdue'
        WHEN g.end_date - $2::date <= 7 THEN 'urgent'
        WHEN g.end_date - $2::date <= 30 THEN 'approaching'
        ELSE 'on_track'
    END AS deadline_status
FROM goals g
WHERE g.user_id = $1 AND g.end_date IS NOT NULL;
```

**Response Fields:**
- `goal_end_date` (string, YYYY-MM-DD)
- `days_until_goal_deadline` (int)
- `deadline_status` (enum: "overdue", "urgent", "approaching", "on_track")

---

### 3. Bounce-Back Rate (Resilience Metric)

**Description:** After a low-karma day (karma < 0.4), how many days on average does it take to return to above-average karma?

**Value Proposition:** Measures emotional resilience and recovery patterns.

**SQL Implementation:**
```sql
WITH low_karma_days AS (
    SELECT local_date
    FROM journal_entries
    WHERE user_id = $1 AND karma < 0.4
),
user_avg AS (
    SELECT AVG(karma) AS avg_karma
    FROM journal_entries
    WHERE user_id = $1
),
recovery_times AS (
    SELECT
        l.local_date AS low_day,
        MIN(j.local_date) - l.local_date AS days_to_recover
    FROM low_karma_days l
    CROSS JOIN user_avg u
    LEFT JOIN journal_entries j
        ON j.user_id = $1
        AND j.local_date > l.local_date
        AND j.karma > u.avg_karma
    GROUP BY l.local_date
)
SELECT AVG(days_to_recover) AS avg_bounce_back_days
FROM recovery_times
WHERE days_to_recover IS NOT NULL;
```

**Response Field:**
- `avg_bounce_back_days` (float) - Lower is better (faster recovery)

**Performance:** Medium complexity, recommend caching daily and serving from cache

---

### 4. Ratings Volatility/Stability Index

**Description:** Standard deviation of karma over the last 30 days. Low = stable performance, High = erratic patterns.

**Value Proposition:** Helps users understand their emotional stability and life consistency.

**SQL Implementation:**
```sql
SELECT
    STDDEV(karma) AS karma_volatility,
    CASE
        WHEN STDDEV(karma) < 0.1 THEN 'very_stable'
        WHEN STDDEV(karma) < 0.2 THEN 'stable'
        WHEN STDDEV(karma) < 0.3 THEN 'moderate'
        ELSE 'volatile'
    END AS stability_category
FROM journal_entries
WHERE user_id = $1
  AND local_date >= $2 - INTERVAL '30 days'
  AND local_date <= $2;
```

**Response Fields:**
- `karma_volatility` (float, 0-1)
- `stability_category` (enum: "very_stable", "stable", "moderate", "volatile")

---

### 5. Positive Days Ratio

**Description:** Percentage of days with karma > 0.5 (above the midpoint).

**Value Proposition:** Simple, powerful indicator of overall well-being and goal alignment.

**SQL Implementation:**
```sql
SELECT
    COUNT(*) FILTER (WHERE karma > 0.5) AS positive_days,
    COUNT(*) AS total_days,
    ROUND(
        (COUNT(*) FILTER (WHERE karma > 0.5)::numeric / NULLIF(COUNT(*), 0)) * 100,
        2
    ) AS positive_days_percentage
FROM journal_entries
WHERE user_id = $1;
```

**Response Fields:**
- `positive_days_ratio` (float, 0-1)
- `positive_days_count` (int)

**Performance:** Fast, single table scan with index

---

### 6. Comeback Count (Resilience Tracker)

**Description:** Number of times user has restarted logging after a gap of 3+ days.

**Value Proposition:** Celebrates resilience and persistence. "You've bounced back 12 times - that's dedication!"

**SQL Implementation:**
```sql
WITH gaps AS (
    SELECT
        local_date,
        local_date - LAG(local_date) OVER (ORDER BY local_date) AS gap_size
    FROM journal_entries
    WHERE user_id = $1
)
SELECT COUNT(*) AS comeback_count
FROM gaps
WHERE gap_size >= 3;
```

**Response Field:**
- `comeback_count` (int)

---

## Tier 3 Metrics - Advanced Features

### 1. Velocity & Acceleration Scores

**Description:**
- **Velocity:** Rate of karma improvement over last 30 days (linear regression slope)
- **Acceleration:** Is velocity itself increasing? (second derivative)

**Value Proposition:** Predictive indicators of trajectory. "You're not just improving, you're improving faster!"

**SQL Implementation:**
```sql
WITH daily_karma AS (
    SELECT
        local_date,
        karma,
        local_date - MIN(local_date) OVER () AS day_number
    FROM journal_entries
    WHERE user_id = $1
      AND local_date >= $2 - INTERVAL '30 days'
      AND local_date <= $2
),
stats AS (
    SELECT
        COUNT(*) AS n,
        SUM(day_number) AS sum_x,
        SUM(karma) AS sum_y,
        SUM(day_number * karma) AS sum_xy,
        SUM(day_number * day_number) AS sum_xx
    FROM daily_karma
)
SELECT
    CASE
        WHEN n * sum_xx - sum_x * sum_x = 0 THEN 0
        ELSE (n * sum_xy - sum_x * sum_y) / (n * sum_xx - sum_x * sum_x)
    END AS velocity_slope
FROM stats;
```

**Response Fields:**
- `karma_velocity` (float) - Positive = improving, negative = declining
- `velocity_trend` (enum: "accelerating", "stable", "decelerating") - Requires comparing 30-day vs 60-day velocity

**Performance:** Moderate - window functions with 30-day window

---

### 2. Alignment-Contentment Gap Analysis

**Description:** Identify when alignment and contentment diverge significantly.

**Value Proposition:** Deep psychological insight - "Are you doing what matters but unhappy? Or happy but misaligned?"

**SQL Implementation:**
```sql
SELECT
    AVG(alignment_rating - contentment_rating) AS avg_gap,
    STDDEV(alignment_rating - contentment_rating) AS gap_volatility,
    COUNT(*) FILTER (WHERE alignment_rating > contentment_rating + 2) AS high_alignment_low_contentment_days,
    COUNT(*) FILTER (WHERE contentment_rating > alignment_rating + 2) AS high_contentment_low_alignment_days
FROM journal_entries
WHERE user_id = $1
  AND local_date >= $2 - INTERVAL '30 days'
  AND local_date <= $2;
```

**Response Fields:**
- `alignment_contentment_gap_avg` (float, -9 to 9)
- `gap_type` (enum: "balanced", "productive_but_unhappy", "happy_but_drifting")

**Performance:** Fast with indexed scan

---

### 3. Seasonal/Monthly Pattern Analysis

**Description:** Identify which months or seasons yield best performance.

**Value Proposition:** "You consistently perform best in Spring" - helps with planning and self-awareness.

**SQL Implementation:**
```sql
SELECT
    EXTRACT(MONTH FROM local_date) AS month,
    TO_CHAR(local_date, 'Month') AS month_name,
    AVG(karma) AS avg_karma,
    COUNT(*) AS entries
FROM journal_entries
WHERE user_id = $1
GROUP BY EXTRACT(MONTH FROM local_date), TO_CHAR(local_date, 'Month')
HAVING COUNT(*) >= 5  -- Only show months with sufficient data
ORDER BY avg_karma DESC;
```

**Response Field:**
- `seasonal_patterns` (array of objects with month, avg_karma)

**Data Requirement:** Minimum 6 months of consistent logging

---

### 4. Predictive Trajectory

**Description:** Based on current velocity and goal deadline, predict if user will hit their target alignment score.

**Value Proposition:** "At your current pace, you'll reach your goal 3 weeks early!" or "You need to increase average alignment by 1.5 points to meet your deadline."

**SQL Implementation:**
```sql
-- Combines velocity calculation with goal deadline
WITH recent_velocity AS (
    -- Use velocity query from above
    SELECT 0.05 AS daily_improvement  -- placeholder
),
goal_info AS (
    SELECT end_date, end_date - $2::date AS days_remaining
    FROM goals WHERE user_id = $1
),
current_avg AS (
    SELECT AVG(alignment_rating) AS current_avg_alignment
    FROM journal_entries
    WHERE user_id = $1
      AND local_date >= $2 - INTERVAL '7 days'
      AND local_date <= $2
)
SELECT
    g.days_remaining,
    c.current_avg_alignment + (v.daily_improvement * g.days_remaining) AS projected_alignment,
    CASE
        WHEN c.current_avg_alignment + (v.daily_improvement * g.days_remaining) >= 8.0
            THEN 'on_track'
        ELSE 'needs_improvement'
    END AS trajectory_status
FROM goal_info g, recent_velocity v, current_avg c;
```

**Response Fields:**
- `projected_alignment_at_deadline` (float)
- `trajectory_status` (enum: "ahead_of_schedule", "on_track", "needs_improvement", "off_track")

**Data Requirement:** Goal with deadline + 14+ days of data

---

### 5. Correlation Discovery Engine

**Description:** Automatically discover correlations between:
- Day of week and specific rating types
- Streak length and karma quality
- Gap length and recovery karma
- Month-start effect (first week vs rest of month)

**Value Proposition:** "Your Tuesdays average 23% higher karma than Mondays" - automated pattern discovery.

**Implementation:** Requires multiple complex queries, recommend background job that runs nightly and caches results.

**Example - Tuesday Effect:**
```sql
SELECT
    EXTRACT(DOW FROM local_date) AS day_of_week,
    TO_CHAR(local_date, 'Day') AS day_name,
    AVG(karma) AS avg_karma,
    AVG(alignment_rating) AS avg_alignment,
    AVG(contentment_rating) AS avg_contentment,
    COUNT(*) AS sample_size
FROM journal_entries
WHERE user_id = $1
GROUP BY EXTRACT(DOW FROM local_date), TO_CHAR(local_date, 'Day')
HAVING COUNT(*) >= 3
ORDER BY avg_karma DESC;
```

**Response Field:**
- `discovered_patterns` (array of pattern objects with description and strength)

---

## Implementation Guidelines

### Phase 1 (Tier 2 - Q1 2026)
1. **Alignment-Contentment Correlation** - Unique psychological insight
2. **Goal Countdown** - High user value if goal has deadline
3. **Positive Days Ratio** - Simple but powerful motivator
4. **Comeback Count** - Celebrates resilience

### Phase 2 (Tier 3 - Q2 2026)
1. **Velocity Score** - Predictive, forward-looking
2. **Seasonal Patterns** (requires 6+ months data for most users)
3. **Day-of-Week Full Breakdown** (expand on Tier 1 peak day)

### Phase 3 (Advanced Analytics - Q3 2026)
1. **Correlation Discovery Engine** - Full automated pattern detection
2. **Predictive Trajectory** - Goal completion forecasting
3. **Alignment-Contentment Gap Analysis** - Deep psychological insights

---

## Performance Considerations

### Query Optimization Strategies

1. **Use CTEs (Common Table Expressions)** for complex calculations
2. **Leverage Materialized Views** for expensive daily aggregations
3. **Cache Results** - Store computed metrics in a `dashboard_cache` table
4. **Index Strategy:**
   - `(user_id, local_date DESC)` - Already implemented ✅
   - `(user_id, created_at)` - Already implemented ✅
   - Future: `(user_id, karma)` for percentile queries

### Caching Strategy ✅ IMPLEMENTED

**Status:** The `user_stats` table has been implemented as of 2025-10-15!

The `user_stats` table provides **real-time incremental updates** instead of periodic batch recalculation:

```sql
-- See internal/db/migrate.go for full schema
CREATE TABLE user_stats (
    user_id INTEGER PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    total_days_logged INTEGER NOT NULL DEFAULT 0,
    sum_alignment_rating REAL NOT NULL DEFAULT 0,
    sum_contentment_rating REAL NOT NULL DEFAULT 0,
    total_karma REAL NOT NULL DEFAULT 0,
    current_streak_days INTEGER NOT NULL DEFAULT 0,
    longest_streak_ever INTEGER NOT NULL DEFAULT 0,
    current_streak_start_date DATE,
    last_entry_date DATE,
    last_week_karma REAL NOT NULL DEFAULT 0,
    day_of_week_stats JSONB DEFAULT '{}'::jsonb,
    positive_days_count INTEGER NOT NULL DEFAULT 0,
    comeback_count INTEGER NOT NULL DEFAULT 0,
    -- ... and more
);
```

**Update Strategy:**
- **On INSERT:** Increment counters, update streaks, recalculate running averages
- **On UPDATE:** Adjust sums incrementally (add delta)
- **On DELETE:** Decrement counters, recalculate streaks if needed (rare operation)

**Benefits:**
- ✅ O(1) read time for dashboard loads
- ✅ Always up-to-date (no staleness)
- ✅ Incremental updates via transactions (ACID guarantees)
- ✅ Scales to millions of users
- ✅ No background jobs needed for most metrics

**Performance Comparison:**

| Metric | Before (Runtime) | After (Precomputed) |
|--------|------------------|---------------------|
| Total Days Logged | O(n) COUNT(*) | O(1) lookup |
| Longest Streak | O(n log n) CTE | O(1) lookup |
| Avg Ratings (All-Time) | O(n) AVG() | O(1) calculation from sums |
| Current Streak | O(n log n) CTE | O(1) lookup |

**Dashboard Query Reduction:**
- Before: 10+ database queries (including expensive CTEs)
- After: 4 queries (1 for user_stats, 3 for time-dependent metrics)
- **~75% reduction in query complexity**

---

## Data Requirements

| Metric | Minimum Data | Recommended Data |
|--------|--------------|------------------|
| Correlation | 7 days | 30 days |
| Volatility | 7 days | 30 days |
| Bounce-Back Rate | 3 low-karma days | 10+ low-karma days |
| Comeback Count | 1 gap | 3+ gaps |
| Seasonal Patterns | 3 months | 12 months |
| Velocity/Acceleration | 14 days | 30 days |
| Predictive Trajectory | 14 days + goal | 30 days + goal |

---

## Example API Response (Future State)

```json
{
  "reference_date": "2025-10-15",
  "day_karma": 0.89,
  "week_karma": 5.23,

  "tier_2_insights": {
    "alignment_contentment_correlation": 0.78,
    "correlation_interpretation": "Strong positive - your happiness and goal alignment move together",
    "goal_countdown": {
      "days_remaining": 45,
      "deadline_status": "on_track"
    },
    "positive_days_ratio": 0.73,
    "comeback_count": 8,
    "bounce_back_rate_days": 1.3,
    "karma_volatility": 0.15,
    "stability_category": "stable"
  },

  "tier_3_insights": {
    "karma_velocity": 0.008,
    "velocity_interpretation": "Improving steadily - +0.24 karma per month",
    "trajectory_status": "ahead_of_schedule",
    "alignment_contentment_gap": {
      "avg_gap": 0.5,
      "pattern": "balanced"
    },
    "discovered_patterns": [
      {
        "type": "day_of_week",
        "insight": "Your Sundays average 31% higher karma than Mondays",
        "confidence": "high"
      },
      {
        "type": "streak_effect",
        "insight": "Streaks longer than 7 days show 18% better karma quality",
        "confidence": "medium"
      }
    ]
  }
}
```

---

## Questions for Product Team

1. **Which Tier 2 metrics should we prioritize first?** (Recommendation: Correlation, Goal Countdown, Positive Days Ratio)
2. **Should we create a separate `/dashboard/insights` endpoint?** Or include everything in main dashboard?
3. **Caching strategy approval?** Add `dashboard_metrics_cache` table or use Redis?
4. **UI/UX considerations?** How do we present correlation coefficients to non-technical users?
5. **Mobile vs. Web?** Are Tier 3 metrics web-only or should mobile also display them?

---

## Contributing

When implementing metrics from this document:
1. Add SQL implementation to `internal/handlers/dashboard.go`
2. Add response fields to `dashboardResponse` struct
3. Update this document's status to "Implemented ✅"
4. Add tests in `dashboard_test.go`
5. Document the metric in API documentation

---

**Document Owner:** Backend Team
**Reviewers:** Product, Design, Data Science
**Next Review Date:** 2025-11-15
