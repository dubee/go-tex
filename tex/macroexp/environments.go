package macroexp

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/luo-studio/go-tex/tex/lexer"
)

type environment struct {
	begin, end string
	args       int
	defaultArg *string
}

func (e *Expander) optional() (string, bool, error) {
	for {
		t := e.pop()
		if t.IsSpace() {
			continue
		}
		if t.Text != "[" {
			e.Push(t)
			return "", false, nil
		}
		break
	}
	var s strings.Builder
	depth := 0
	for {
		t := e.pop()
		if t.IsEOF() {
			return "", true, fmt.Errorf("unterminated optional argument")
		}
		if t.Text == "]" && depth == 0 {
			return s.String(), true, nil
		}
		if t.Text == "{" {
			depth++
		}
		if t.Text == "}" {
			depth--
		}
		s.WriteString(t.Text)
	}
}
func defineEnvironment(renew bool) FnMacro {
	return func(e *Expander) ([]lexer.Token, error) {
		name, err := e.consumeArgs(1)
		if err != nil {
			return nil, err
		}
		if name[0] == "" {
			return nil, fmt.Errorf("empty environment name")
		}
		exists := false
		if _, ok := e.environments[name[0]]; ok {
			exists = true
		}
		if exists != renew {
			return nil, fmt.Errorf("environment definition conflict: %s", name[0])
		}
		n, defaultArg, err := e.argumentSpec()
		if err != nil {
			return nil, err
		}
		body, err := e.consumeArgs(2)
		if err != nil {
			return nil, err
		}
		if err := validateParameters(lexBodySource(body[0]), n); err != nil {
			return nil, err
		}
		// End code cannot take parameters in LaTeX; reject rather than silently drop them.
		if err := validateParameters(lexBodySource(body[1]), 0); err != nil {
			return nil, err
		}
		e.environments[name[0]] = environment{body[0], body[1], n, defaultArg}
		return nil, nil
	}
}
func expandEnvironment(begin bool) FnMacro {
	return func(e *Expander) ([]lexer.Token, error) {
		name, err := e.consumeArgs(1)
		if err != nil {
			return nil, err
		}
		def, ok := e.environments[name[0]]
		if !ok {
			cmd := `\end`
			if begin {
				cmd = `\begin`
			}
			tokens := lexAll(cmd + "{" + name[0] + "}")
			tokens[len(tokens)-1].NoExpand = true
			return tokens, nil
		}
		if begin {
			var first []string
			count := def.args
			if def.defaultArg != nil {
				raw, has, err := e.optional()
				if err != nil {
					return nil, err
				}
				if !has {
					raw = *def.defaultArg
				}
				first = []string{raw}
				count--
			}
			args, err := e.consumeArgs(count)
			args = append(first, args...)
			if err != nil {
				return nil, err
			}
			s, err := substituteArgs(def.begin, args)
			if err != nil {
				return nil, err
			}
			e.activeEnvs = append(e.activeEnvs, name[0])
			return lexAll(s), nil
		}
		if len(e.activeEnvs) == 0 || e.activeEnvs[len(e.activeEnvs)-1] != name[0] {
			return nil, fmt.Errorf("mismatched custom environment: %s", name[0])
		}
		e.activeEnvs = e.activeEnvs[:len(e.activeEnvs)-1]
		return lexAll(def.end), nil
	}
}

// argumentSpec reads exactly [count][default], preserving an empty default.
func (e *Expander) argumentSpec() (int, *string, error) {
	raw, has, err := e.optional()
	if err != nil || !has {
		return 0, nil, err
	}
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || n < 0 || n > 9 {
		return 0, nil, fmt.Errorf("invalid macro argument count")
	}
	value, has, err := e.optional()
	if err != nil {
		return 0, nil, err
	}
	if has {
		if n == 0 {
			return 0, nil, fmt.Errorf("optional default requires at least one argument")
		}
		return n, &value, nil
	}
	return n, nil, nil
}
func validateParameters(body []lexer.Token, count int) error {
	for i := 0; i < len(body); i++ {
		if body[i].Text != "#" {
			continue
		}
		i++
		if i >= len(body) {
			return fmt.Errorf("incomplete macro parameter")
		}
		text := body[i].Text
		if text == "#" {
			continue
		}
		if len(text) != 1 || text[0] < '1' || text[0] > '9' || int(text[0]-'0') > count {
			return fmt.Errorf("invalid macro parameter #%s for %d arguments", text, count)
		}
	}
	return nil
}
