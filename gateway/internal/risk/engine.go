package risk

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sync"
)

type RiskLevel string

const (
	RiskLevelLow    RiskLevel = "low"
	RiskLevelMedium RiskLevel = "medium"
	RiskLevelHigh  RiskLevel = "high"
)

type Action string

const (
	ActionAllow    Action = "allow"
	ActionBlock   Action = "block"
	ActionConfirm Action = "confirm"
)

type Rule struct {
	ID      string     `json:"id"`
	Pattern *regexp.Regexp
	Level   RiskLevel `json:"level"`
	Action  Action    `json:"action"`
	Message string    `json:"message"`
}

type RuleConfig struct {
	ID      string    `json:"id"`
	Pattern string    `json:"pattern"`
	Level   RiskLevel `json:"level"`
	Action  Action   `json:"action"`
	Message string    `json:"message"`
}

type Engine struct {
	rules    []Rule
	mu       sync.RWMutex
}

func NewEngine() *Engine {
	return &Engine{
		rules: make([]Rule, 0),
	}
}

// LoadRules loads rules from a JSON file
func (e *Engine) LoadRules(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read rules file: %w", err)
	}
	return e.LoadRulesFromJSON(data)
}

// RulesFileWrapper is used to parse JSON files with the risk_rules wrapper
type RulesFileWrapper struct {
	RiskRules []RuleConfig `json:"risk_rules"`
}

// LoadRulesFromJSON loads rules from JSON data
func (e *Engine) LoadRulesFromJSON(data []byte) error {
	var configs []RuleConfig

	// Try to parse as wrapped format first (for JSON files)
	var wrapper RulesFileWrapper
	if err := json.Unmarshal(data, &wrapper); err == nil && len(wrapper.RiskRules) > 0 {
		configs = wrapper.RiskRules
	} else {
		// Fall back to direct array format (for AddDefaultRules)
		if err := json.Unmarshal(data, &configs); err != nil {
			return fmt.Errorf("parse rules JSON: %w", err)
		}
	}

	rules := make([]Rule, 0, len(configs))
	for _, cfg := range configs {
		re, err := regexp.Compile(cfg.Pattern)
		if err != nil {
			return fmt.Errorf("compile pattern %s: %w", cfg.ID, err)
		}
		rules = append(rules, Rule{
			ID:      cfg.ID,
			Pattern: re,
			Level:   cfg.Level,
			Action:  cfg.Action,
			Message: cfg.Message,
		})
	}

	e.mu.Lock()
	e.rules = rules
	e.mu.Unlock()

	return nil
}

// Check evaluates a command and returns the matching rule and risk level
func (e *Engine) Check(command string) (*Rule, RiskLevel, Action) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	for _, rule := range e.rules {
		if rule.Pattern.MatchString(command) {
			return &rule, rule.Level, rule.Action
		}
	}

	return nil, RiskLevelLow, ActionAllow
}

// CheckAndBlock evaluates command and returns whether to block
func (e *Engine) CheckAndBlock(command string) (bool, *Rule) {
	rule, _, action := e.Check(command)
	if action == ActionBlock {
		return true, rule
	}
	return false, nil
}

// CheckAndConfirm evaluates command and returns whether to require confirmation
func (e *Engine) CheckAndConfirm(command string) (bool, *Rule) {
	rule, _, action := e.Check(command)
	if action == ActionConfirm {
		return true, rule
	}
	return false, nil
}

// GetBlockMessage returns the formatted block message for a rule
func (e *Engine) GetBlockMessage(rule *Rule) string {
	if rule == nil {
		return ""
	}
	return fmt.Sprintf("\r\n\x1b[31m[BLOCKED]\x1b[0m %s\r\n", rule.Message)
}

// GetConfirmMessage returns the formatted confirm message for a rule
func (e *Engine) GetConfirmMessage(rule *Rule) string {
	if rule == nil {
		return ""
	}
	return fmt.Sprintf("\r\n\x1b[33m[CONFIRM]\x1b[0m %s\r\n", rule.Message)
}

// AddDefaultRules loads the default risk rules
func (e *Engine) AddDefaultRules() error {
	defaultRules := []RuleConfig{
		{
			ID:      "rule_rm_rf_root",
			Pattern: `^rm\s+-r[fF]?\s+/(?!\w)`,
			Level:   RiskLevelHigh,
			Action:  ActionBlock,
			Message: "高危操作拦截：禁止删除根目录或关键系统目录",
		},
		{
			ID:      "rule_service_restart",
			Pattern: `^(systemctl|service)\s+(restart|stop|halt)`,
			Level:   RiskLevelMedium,
			Action:  ActionConfirm,
			Message: "您正在尝试停止或重启基础服务，请确认该操作不影响业务",
		},
		{
			ID:      "rule_mkfs",
			Pattern: `^mkfs`,
			Level:   RiskLevelHigh,
			Action:  ActionBlock,
			Message: "高危操作拦截：禁止执行 mkfs",
		},
		{
			ID:      "rule_reboot",
			Pattern: `^reboot|^shutdown|^init\s+0|^init\s+6`,
			Level:   RiskLevelMedium,
			Action:  ActionConfirm,
			Message: "系统将重启，请确认操作",
		},
	}

	data, _ := json.Marshal(defaultRules)
	return e.LoadRulesFromJSON(data)
}