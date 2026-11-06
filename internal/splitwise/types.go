package splitwise

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// User is a Splitwise user as returned by get_current_user / group members.
type User struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email"`
}

func (u User) DisplayName() string {
	name := strings.TrimSpace(u.FirstName + " " + u.LastName)
	if name == "" {
		return u.Email
	}
	return name
}

// Balance is a currency amount on a group member.
type Balance struct {
	CurrencyCode string `json:"currency_code"`
	Amount       string `json:"amount"`
}

// Member is a group member with balances.
type Member struct {
	User
	Balance []Balance `json:"balance"`
}

// Group is a Splitwise group.
type Group struct {
	ID      int64    `json:"id"`
	Name    string   `json:"name"`
	Members []Member `json:"members"`
}

// Expense is a subset of a Splitwise expense used for listing and create results.
type Expense struct {
	ID          int64  `json:"id"`
	Cost        string `json:"cost"`
	Description string `json:"description"`
	Currency    string `json:"currency_code"`
	GroupID     int64  `json:"group_id"`
	Date        string `json:"date"`
}

// Share is one person's paid/owed amounts on an expense.
type Share struct {
	UserID    int64
	PaidShare string
	OwedShare string
}

// CreateExpenseInput is the body for POST /create_expense.
type CreateExpenseInput struct {
	Cost         string
	Description  string
	Details      string
	Date         string
	CurrencyCode string
	GroupID      int64
	SplitEqually bool
	Shares       []Share
}

func (in CreateExpenseInput) validate() error {
	if strings.TrimSpace(in.Cost) == "" {
		return fmt.Errorf("cost is required")
	}
	if strings.TrimSpace(in.Description) == "" {
		return fmt.Errorf("description is required")
	}
	if in.SplitEqually && len(in.Shares) > 0 {
		return fmt.Errorf("split equally and explicit shares are mutually exclusive")
	}
	if !in.SplitEqually && len(in.Shares) == 0 {
		return fmt.Errorf("provide split equally or at least one share")
	}
	for i, s := range in.Shares {
		if s.UserID == 0 {
			return fmt.Errorf("share %d: user id is required", i)
		}
		if s.PaidShare == "" || s.OwedShare == "" {
			return fmt.Errorf("share %d: paid and owed amounts are required", i)
		}
	}
	return nil
}

func (in CreateExpenseInput) payload() (map[string]any, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}
	m := map[string]any{
		"cost":        in.Cost,
		"description": in.Description,
		"group_id":    in.GroupID,
	}
	if in.Details != "" {
		m["details"] = in.Details
	}
	if in.Date != "" {
		m["date"] = in.Date
	}
	if in.CurrencyCode != "" {
		m["currency_code"] = in.CurrencyCode
	}
	if in.SplitEqually {
		m["split_equally"] = true
		return m, nil
	}
	for i, s := range in.Shares {
		m[fmt.Sprintf("users__%d__user_id", i)] = s.UserID
		m[fmt.Sprintf("users__%d__paid_share", i)] = s.PaidShare
		m[fmt.Sprintf("users__%d__owed_share", i)] = s.OwedShare
	}
	return m, nil
}

type currentUserResponse struct {
	User User `json:"user"`
}

type groupsResponse struct {
	Groups []Group `json:"groups"`
}

type expensesResponse struct {
	Expenses []Expense `json:"expenses"`
}

type createExpenseResponse struct {
	Expenses []Expense       `json:"expenses"`
	Errors   json.RawMessage `json:"errors"`
}

func apiErrors(raw json.RawMessage) error {
	if len(raw) == 0 {
		return nil
	}
	s := strings.TrimSpace(string(raw))
	if s == "" || s == "null" || s == "{}" || s == "[]" {
		return nil
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err == nil {
		var parts []string
		for k, v := range obj {
			switch t := v.(type) {
			case []any:
				if len(t) == 0 {
					continue
				}
				msgs := make([]string, 0, len(t))
				for _, item := range t {
					msgs = append(msgs, fmt.Sprint(item))
				}
				parts = append(parts, k+": "+strings.Join(msgs, "; "))
			case string:
				if t != "" {
					parts = append(parts, k+": "+t)
				}
			}
		}
		if len(parts) == 0 {
			return nil
		}
		return fmt.Errorf("splitwise errors: %s", strings.Join(parts, "; "))
	}
	var arr []string
	if err := json.Unmarshal(raw, &arr); err == nil {
		if len(arr) == 0 {
			return nil
		}
		return fmt.Errorf("splitwise errors: %s", strings.Join(arr, "; "))
	}
	return fmt.Errorf("splitwise errors: %s", s)
}

// FindGroup matches q as a numeric id or a case-insensitive group name.
func FindGroup(groups []Group, q string) (Group, error) {
	q = strings.TrimSpace(q)
	if q == "" {
		return Group{}, fmt.Errorf("group is required")
	}
	if id, err := strconv.ParseInt(q, 10, 64); err == nil {
		for _, g := range groups {
			if g.ID == id {
				return g, nil
			}
		}
		return Group{}, fmt.Errorf("no group with id %d", id)
	}
	var matches []Group
	lower := strings.ToLower(q)
	for _, g := range groups {
		if strings.ToLower(g.Name) == lower {
			matches = append(matches, g)
		}
	}
	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		return Group{}, fmt.Errorf("no group named %q", q)
	default:
		return Group{}, fmt.Errorf("multiple groups named %q; use the numeric id", q)
	}
}
