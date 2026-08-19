# Persistent learner profile

Step 4 of I-02 adds one local learner profile per workspace. The profile exists
only to personalize learning; it is not a social identity and does not collect
age, gender, address, credentials, or other unnecessary sensitive data.

## Domain and defaults

`internal/learning.StudentProfile` owns validation. The profile contains:

- an optional display name;
- a coarse, general experience level (`novice`, `beginner`, `intermediate`, or
  `advanced`), distinct from future goal-specific diagnosis;
- a preferred language tag;
- a daily study-time budget and weekly-days target;
- zero or more subject-neutral learning preferences;
- an IANA timezone.

First use creates the stable workspace learner `student.primary` with
deterministic defaults: no display name, `novice`, `en`, 30 minutes per day,
five days per week, no learning preferences, and `UTC`. Kelyro deliberately
does not infer locale, name, or timezone from the host.

`application.ProfileService` owns create-on-first-use and partial edits. CLI and
TUI never access repositories or SQLite directly. Profile timestamps are UTC;
edits preserve the stable student ID and creation timestamp.

## Persistence compatibility

Migration v5 is forward-only and adds profile columns to the v4
`student_profiles` table. Existing v4 rows retain their name and derive daily
minutes from their saved weekly budget and preferred-day count. Language and
timezone receive the deterministic defaults.

The published v4 `display_name` and `weekly_minutes` columns cannot be removed
or have their checks changed. They remain synchronized compatibility mirrors.
The nullable `preferred_display_name` column is authoritative, allowing an
empty optional display name without rewriting v4. A later destructive cleanup
may remove the mirrors only through a new migration with the required backup.

## Presentation

The CLI exposes human-readable commands:

```text
kelyro profile show
kelyro profile edit --display-name Ada --experience intermediate
kelyro profile edit --language es-PE --timezone America/Lima
kelyro profile edit --daily-minutes 45 --weekly-days 4
kelyro profile edit --learning-styles practice,reflection
```

`--display-name=` and `--learning-styles=` clear those optional values. The TUI
home screen links to a simple read-only profile view with refresh and directs
the learner to the CLI for edits. Onboarding and goal-specific diagnosis remain
future steps.
