//go:build windows && wails

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"icicle/internal/organize"
)

type RouteRule struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Enabled  bool   `json:"enabled"`
	Kind     string `json:"kind"` // ext|contains|prefix|regex
	Pattern  string `json:"pattern"`
	Target   string `json:"target"`
	Priority int    `json:"priority"`
}

type RouteMatch struct {
	Matched bool   `json:"matched"`
	RuleID  string `json:"ruleId"`
	Rule    string `json:"rule"`
	Target  string `json:"target"`
}

func (a *App) routeRulesPath() string {
	cfgDir, _ := os.UserConfigDir()
	if strings.TrimSpace(cfgDir) == "" {
		cfgDir = a.folders.Home
	}
	return filepath.Join(cfgDir, "icicle", "routing_rules.json")
}

func (a *App) ListRoutingRules() ([]RouteRule, error) {
	path := a.routeRulesPath()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return []RouteRule{}, nil
	}
	if err != nil {
		return nil, err
	}
	var rules []RouteRule
	if err := json.Unmarshal(data, &rules); err != nil {
		return nil, err
	}
	out := normalizeRouteRules(rules)
	return out, nil
}

func (a *App) SaveRoutingRules(rules []RouteRule) error {
	rules = normalizeRouteRules(rules)
	data, err := json.MarshalIndent(rules, "", "  ")
	if err != nil {
		return err
	}
	path := a.routeRulesPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return err
	}
	a.appendLog(fmt.Sprintf("[routing] rules saved: %d", len(rules)))
	return nil
}

func (a *App) TestRouting(path string) (RouteMatch, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return RouteMatch{}, fmt.Errorf("path is required")
	}
	rules, err := a.ListRoutingRules()
	if err != nil {
		return RouteMatch{}, err
	}
	for _, r := range rules {
		if !r.Enabled {
			continue
		}
		if routeRuleMatches(r, path) {
			target := expandRouteTarget(r.Target, a.folders.Home)
			if target == "" {
				continue
			}
			return RouteMatch{Matched: true, RuleID: r.ID, Rule: r.Name, Target: target}, nil
		}
	}
	if auto, ok := organize.DestinationDir(a.folders.Home, path); ok {
		return RouteMatch{Matched: true, RuleID: "builtin", Rule: "builtin-extension", Target: auto}, nil
	}
	return RouteMatch{}, nil
}

func (a *App) resolveAutoDestination(src string) (string, bool) {
	rules, err := a.ListRoutingRules()
	if err == nil {
		for _, r := range rules {
			if !r.Enabled {
				continue
			}
			if !routeRuleMatches(r, src) {
				continue
			}
			target := expandRouteTarget(r.Target, a.folders.Home)
			if strings.TrimSpace(target) == "" {
				continue
			}
			return target, true
		}
	}
	return organize.DestinationDir(a.folders.Home, src)
}

func normalizeRouteRules(in []RouteRule) []RouteRule {
	out := make([]RouteRule, 0, len(in))
	seen := map[string]bool{}
	for i, r := range in {
		r.Name = strings.TrimSpace(r.Name)
		r.Kind = strings.ToLower(strings.TrimSpace(r.Kind))
		r.Pattern = strings.TrimSpace(r.Pattern)
		r.Target = strings.TrimSpace(r.Target)
		if r.Name == "" {
			r.Name = fmt.Sprintf("Rule %d", i+1)
		}
		if r.Kind == "" {
			r.Kind = "ext"
		}
		if r.ID == "" {
			r.ID = fmt.Sprintf("rule-%d", i+1)
		}
		if seen[strings.ToLower(r.ID)] {
			continue
		}
		if r.Pattern == "" || r.Target == "" {
			continue
		}
		seen[strings.ToLower(r.ID)] = true
		out = append(out, r)
	}
	return out
}

func routeRuleMatches(rule RouteRule, path string) bool {
	raw := strings.TrimSpace(path)
	if raw == "" {
		return false
	}
	p := strings.ToLower(raw)
	pat := strings.ToLower(strings.TrimSpace(rule.Pattern))
	if pat == "" {
		return false
	}
	switch rule.Kind {
	case "contains":
		return strings.Contains(p, pat)
	case "prefix":
		return strings.HasPrefix(p, pat)
	case "regex":
		re, err := regexp.Compile(rule.Pattern)
		if err != nil {
			return false
		}
		return re.MatchString(raw)
	default:
		ext := strings.ToLower(filepath.Ext(raw))
		if !strings.HasPrefix(pat, ".") {
			pat = "." + pat
		}
		return ext == pat
	}
}

func expandRouteTarget(target string, home string) string {
	target = strings.TrimSpace(target)
	if target == "" {
		return ""
	}
	target = os.ExpandEnv(target)
	target = strings.ReplaceAll(target, "{home}", home)
	target = filepath.Clean(target)
	return target
}
