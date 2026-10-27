package config

import (
	"fmt"
	"strconv"

	"github.com/dlclark/regexp2"
)

// BulkEditRule defines an edit operation on a specific connection field.
type BulkEditRule struct {
	Field      string `json:"field"`      // "title", "host", "port", "user", "description", "method"
	NewValue   string `json:"newValue"`   // replacement text or direct value
	UseRegex   bool   `json:"useRegex"`   // whether to apply regex search & replace
	MatchRegex string `json:"matchRegex"` // regex pattern to search
}

// BulkEditResult reports the count of modified connections and any warnings.
type BulkEditResult struct {
	ModifiedCount int      `json:"modifiedCount"`
	Errors        []string `json:"errors,omitempty"`
}

// ApplyBulkEdit applies bulk editing rules across specified connections without eval (R3).
func (s *Store) ApplyBulkEdit(connIDs []string, rules []BulkEditRule) (BulkEditResult, error) {
	res := BulkEditResult{}
	if len(connIDs) == 0 || len(rules) == 0 {
		return res, nil
	}

	idMap := make(map[string]bool)
	for _, id := range connIDs {
		idMap[id] = true
	}

	var shouldSave bool
	s.mu.Lock()
	for i := range s.cfg.Connections {
		conn := &s.cfg.Connections[i]
		if !idMap[conn.ID] || conn.Protected {
			continue
		}

		connChanged := false
		for _, rule := range rules {
			changed, err := applyRuleToConnection(conn, rule)
			if err != nil {
				res.Errors = append(res.Errors, fmt.Sprintf("Conn '%s' field '%s': %v", conn.Title, rule.Field, err))
			} else if changed {
				connChanged = true
			}
		}

		if connChanged {
			res.ModifiedCount++
			shouldSave = true
		}
	}
	s.mu.Unlock()

	if shouldSave {
		_ = s.Save()
	}

	return res, nil
}

func applyRuleToConnection(conn *Connection, rule BulkEditRule) (bool, error) {
	switch rule.Field {
	case "title":
		val, changed, err := transformString(conn.Title, rule)
		if err == nil && changed {
			conn.Title = val
			return true, nil
		}
		return false, err

	case "host":
		val, changed, err := transformString(conn.Host, rule)
		if err == nil && changed {
			conn.Host = val
			return true, nil
		}
		return false, err

	case "user":
		val, changed, err := transformString(conn.Auth.User, rule)
		if err == nil && changed {
			conn.Auth.User = val
			return true, nil
		}
		return false, err

	case "port":
		if rule.UseRegex {
			portStr := strconv.Itoa(conn.Port)
			val, changed, err := transformString(portStr, rule)
			if err != nil {
				return false, err
			}
			if changed {
				p, err := strconv.Atoi(val)
				if err != nil || p <= 0 || p > 65535 {
					return false, fmt.Errorf("invalid port value: %s", val)
				}
				conn.Port = p
				return true, nil
			}
			return false, nil
		}
		p, err := strconv.Atoi(rule.NewValue)
		if err != nil || p <= 0 || p > 65535 {
			return false, fmt.Errorf("invalid port value: %s", rule.NewValue)
		}
		if conn.Port != p {
			conn.Port = p
			return true, nil
		}
		return false, nil

	case "description":
		val, changed, err := transformString(conn.Description, rule)
		if err == nil && changed {
			conn.Description = val
			return true, nil
		}
		return false, err

	case "method":
		if rule.UseRegex {
			val, changed, err := transformString(conn.Method, rule)
			if err == nil && changed {
				conn.Method = val
				return true, nil
			}
			return false, err
		}
		if conn.Method != rule.NewValue {
			conn.Method = rule.NewValue
			return true, nil
		}
		return false, nil

	default:
		return false, fmt.Errorf("unsupported field: %s", rule.Field)
	}
}

func transformString(current string, rule BulkEditRule) (string, bool, error) {
	if !rule.UseRegex {
		if current != rule.NewValue {
			return rule.NewValue, true, nil
		}
		return current, false, nil
	}

	if rule.MatchRegex == "" {
		return current, false, nil
	}

	re, err := regexp2.Compile(rule.MatchRegex, regexp2.None)
	if err != nil {
		return current, false, fmt.Errorf("invalid regex '%s': %w", rule.MatchRegex, err)
	}

	res, err := re.Replace(current, rule.NewValue, -1, -1)
	if err != nil {
		return current, false, fmt.Errorf("replace error: %w", err)
	}

	return res, res != current, nil
}
