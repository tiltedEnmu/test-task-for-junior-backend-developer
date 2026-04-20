package recurrencerule

import (
    "time"
)

type Parity int32

const (
    ParityUnset Parity = iota
    ParityOdd
    ParityEven
)

type RecurrenceRule struct {
    IsEveryday    bool        `json:"is_everyday"`
    Parity        Parity      `json:"parity"`
    DaysOfWeek    []int32     `json:"days_of_week"`
    DaysOfMonth   []int32     `json:"days_of_month"`
    SpecificDates []time.Time `json:"specific_dates"`
    NextRunAt     time.Time   `json:"next_run_at"`
    IsExpired     bool        `json:"is_expired"`
    StartRuleDate time.Time   `json:"start_rule_date"`
    EndRuleDate   time.Time   `json:"end_rule_date"`
}

func (parity Parity) Valid() bool {
    switch parity {
    case ParityUnset, ParityOdd, ParityEven:
        return true
    default:
        return false
    }
}

func (rule RecurrenceRule) Valid() bool {
    if !rule.Parity.Valid() {
        return false
    }
    if len(rule.DaysOfWeek) > 7 {
        return false
    }
    if len(rule.DaysOfMonth) > 31 {
        return false
    }
    if rule.StartRuleDate.IsZero() {
        rule.StartRuleDate = time.Now().UTC()
    }
    rule.StartRuleDate = dateOnly(rule.StartRuleDate)
    rule.EndRuleDate = dateOnly(rule.EndRuleDate)
    if !rule.EndRuleDate.IsZero() && (rule.EndRuleDate.Before(rule.StartRuleDate)) {
        return false
    }
    if !rule.EndRuleDate.IsZero() && (rule.EndRuleDate.Sub(rule.StartRuleDate) > 365*24*time.Hour) {
        return false
    }
    for _, v := range rule.DaysOfWeek {
        if v < 1 || v > 7 {
            return false
        }
    }
    for _, v := range rule.DaysOfMonth {
        if v < 1 || v > 31 {
            return false
        }
    }
    for _, v := range rule.SpecificDates {
        if v.Before(rule.StartRuleDate) ||
            (!rule.EndRuleDate.IsZero() && v.After(rule.EndRuleDate)) {
            return false
        }
    }
    if hasDuplicates(rule.DaysOfWeek) {
        return false
    }
    if hasDuplicates(rule.DaysOfMonth) {
        return false
    }
    if hasDuplicates(rule.SpecificDates) {
        return false
    }

    return true
}

func (rule RecurrenceRule) Empty() bool {
    return rule.IsEveryday == false &&
        rule.Parity == ParityUnset &&
        len(rule.DaysOfWeek) == 0 &&
        len(rule.DaysOfMonth) == 0 &&
        len(rule.SpecificDates) == 0
}

func Normalize(rule *RecurrenceRule) {
    // for everyday
    if rule.IsEveryday || len(rule.DaysOfWeek) == 7 || len(rule.DaysOfMonth) == 31 {
        rule.IsEveryday = true
        rule.Parity = ParityUnset
        rule.DaysOfWeek = nil
        rule.DaysOfMonth = nil
        rule.SpecificDates = nil
    }

    if rule.StartRuleDate.IsZero() {
        rule.StartRuleDate = time.Now().UTC()
    }
}

func CalcNextRunAt(rule *RecurrenceRule, from time.Time) (time.Time, bool) {
    start := dateOnly(rule.StartRuleDate)
    end := dateOnly(rule.EndRuleDate)
    candidate := dateOnly(from)

    if candidate.Before(start) {
        candidate = start
    }

    specific := make(map[string]struct{}, len(rule.SpecificDates))
    for _, v := range rule.SpecificDates {
        specific[dateOnly(v).Format("2006-01-02")] = struct{}{}
    }

    for {
        if !end.IsZero() && candidate.After(end) {
            return time.Time{}, false
        }

        if rule.IsEveryday {
            return candidate, true
        }

        if _, ok := specific[candidate.Format("2006-01-02")]; ok {
            return candidate, true
        }

        if matchesParity(candidate, rule.Parity) {
            return candidate, true
        }

        if containsInt32(rule.DaysOfWeek, int32(candidate.Weekday())) {
            return candidate, true
        }

        if containsInt32(rule.DaysOfMonth, int32(candidate.Day())) {
            return candidate, true
        }

        candidate = candidate.AddDate(0, 0, 1)
    }
}

// utility
func dateOnly(t time.Time) time.Time {
    if t.IsZero() {
        return time.Time{}
    }
    y, m, d := t.Date()
    return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

func containsInt32(xs []int32, v int32) bool {
    for _, x := range xs {
        if x == v {
            return true
        }
    }
    return false
}

func matchesParity(t time.Time, p Parity) bool {
    switch p {
    case ParityOdd:
        return t.Day()%2 == 1
    case ParityEven:
        return t.Day()%2 == 0
    default:
        return false
    }
}
func hasDuplicates[T comparable](s []T) bool {
    seen := make(map[T]struct{}, len(s))
    for _, v := range s {
        if _, ok := seen[v]; ok {
            return true
        }
        seen[v] = struct{}{}
    }
    return false
}
