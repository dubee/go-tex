package parser

import (
	"github.com/luo-studio/go-tex/tex/colors"
	"math"
	"strconv"
	"strings"
)

// MultiScripts preserves independent left and right scripts as native layout nodes.
type MultiScripts struct {
	LeftAligned                                bool // sideset uses MathJax scriptalign="left left"
	Mode                                       Mode
	Base, LeftSup, LeftSub, RightSup, RightSub Node
}

func (n *MultiScripts) NodeType() string { return "multiscripts" }
func (n *MultiScripts) NodeMode() Mode   { return n.Mode }

type CancelTo struct {
	Mode         Mode
	Body, Target Node
}

func (n *CancelTo) NodeType() string { return "cancelto" }
func (n *CancelTo) NodeMode() Mode   { return n.Mode }
func parseMultiScripts(p *Parser) (Node, error) {
	cmd := p.cur.Text
	p.advance()
	a, err := parseFunctionArg(p, cmd)
	if err != nil {
		return nil, err
	}
	b, err := parseFunctionArg(p, cmd)
	if err != nil {
		return nil, err
	}
	base, err := parseFunctionArg(p, cmd)
	if err != nil {
		return nil, err
	}
	n := &MultiScripts{Mode: p.mode, Base: base}
	if cmd == `\prescript` {
		n.LeftSup, n.LeftSub = a, b
		return n, nil
	}
	extract := func(v Node) (Node, Node, error) {
		v = NormalizeArgument(v)
		if g, ok := v.(*OrdGroup); ok && len(g.Body) == 0 {
			return nil, nil, nil
		}
		s, ok := v.(*SupSub)
		if !ok {
			return nil, nil, errMsg("sideset arguments must contain only scripts")
		}
		if s.Base != nil {
			if g, ok := s.Base.(*OrdGroup); !ok || len(g.Body) != 0 {
				return nil, nil, errMsg("sideset arguments must contain only scripts")
			}
		}
		return s.Sup, s.Sub, nil
	}
	n.LeftAligned = true
	n.LeftSup, n.LeftSub, err = extract(a)
	if err != nil {
		return nil, err
	}
	n.RightSup, n.RightSub, err = extract(b)
	return n, err
}
func parseCancelTo(p *Parser) (Node, error) {
	p.advance()
	target, err := parseFunctionArg(p, `\cancelto`)
	if err != nil {
		return nil, err
	}
	body, err := parseFunctionArg(p, `\cancelto`)
	if err != nil {
		return nil, err
	}
	return &CancelTo{Mode: p.mode, Body: body, Target: target}, nil
}

// Multline aligns the first row left, interior rows centrally and the last right.
type Multline struct {
	Mode Mode
	Rows []Node
}

func (n *Multline) NodeType() string { return "multline" }
func (n *Multline) NodeMode() Mode   { return n.Mode }
func parseMultline(p *Parser) (Node, error) {
	rows, _, _, err := parseArrayBody(p, StyleDisplay)
	if err != nil {
		return nil, err
	}
	n := &Multline{Mode: p.mode}
	for _, row := range rows {
		if len(row) != 1 {
			return nil, errMsg("multline rows cannot contain alignment tabs")
		}
		n.Rows = append(n.Rows, row[0])
	}
	return n, nil
}

// Unicode is an explicit scalar; layout selects only an embedded font.
type Unicode struct {
	Mode  Mode
	Value rune
}

func (n *Unicode) NodeType() string { return "unicode" }
func (n *Unicode) NodeMode() Mode   { return n.Mode }
func parseUnicode(p *Parser) (Node, error) {
	p.advance()
	raw, err := readEnvName(p, `\unicode`)
	if err != nil {
		return nil, err
	}
	base := 10
	if strings.HasPrefix(raw, "x") || strings.HasPrefix(raw, "X") {
		base = 16
		raw = raw[1:]
	}
	v, err := strconv.ParseInt(raw, base, 32)
	if err != nil || v < 0 || v > 0x10ffff || v >= 0xd800 && v <= 0xdfff {
		return nil, errMsg("invalid Unicode scalar")
	}
	return &Unicode{Mode: p.mode, Value: rune(v)}, nil
}

type Decorated struct {
	Enclosure               bool // True for MathML menclose, distinct from CSS bbox.
	Mode                    Mode
	Body                    Node
	Notation                string
	Padding, Border         float64
	Background, BorderColor string
	PaddingSpec, BorderSpec string
}

func (n *Decorated) NodeType() string { return "decorated" }
func (n *Decorated) NodeMode() Mode   { return n.Mode }
func parseDecorated(p *Parser) (Node, error) {
	cmd := p.cur.Text
	p.advance()
	n := &Decorated{Mode: p.mode, Padding: .2335, Border: .067, BorderColor: ""}
	if cmd == `\enclose` {
		n.Enclosure = true
		raw, err := readEnvName(p, cmd)
		if err != nil {
			return nil, err
		}
		n.Notation = strings.ReplaceAll(raw, ",", " ")
		for _, v := range strings.Fields(n.Notation) {
			switch v {
			case "circle", "box", "roundedbox", "updiagonalstrike", "downdiagonalstrike", "verticalstrike", "horizontalstrike", "top", "bottom", "left", "right", "updiagonalarrow":
			default:
				return nil, errMsg("unsupported enclosure notation: " + v)
			}
		}
	} else {
		n.Padding = 0
		n.Border = 0
		p.consumeSpaces()
		if p.cur.Text == "[" {
			p.advance()
			var raw strings.Builder
			for !p.cur.IsEOF() && p.cur.Text != "]" {
				raw.WriteString(p.cur.Text)
				p.advance()
			}
			if err := p.expect("]"); err != nil {
				return nil, err
			}
			for _, part := range strings.Split(raw.String(), ",") {
				part = strings.TrimSpace(part)
				if strings.HasPrefix(part, "border:") {
					f := strings.Fields(strings.TrimPrefix(part, "border:"))
					if len(f) != 3 || f[1] != "solid" {
						return nil, errMsg("bbox requires border:WIDTH solid COLOR")
					}
					v, err := bboxLength(f[0])
					if err != nil {
						return nil, err
					}
					n.Border = v
					n.BorderSpec = f[0]
					n.BorderColor = f[2]
					n.Notation = "box"
				} else if strings.HasSuffix(part, "px") || strings.HasSuffix(part, "em") || strings.HasSuffix(part, "pt") {
					v, err := bboxLength(part)
					if err != nil {
						return nil, err
					}
					n.Padding = v
					n.PaddingSpec = part
				} else if part != "" {
					n.Background = part
				}
			}
		}
	}
	body, err := parseFunctionArg(p, cmd)
	if err != nil {
		return nil, err
	}
	for _, c := range []string{n.Background, n.BorderColor} {
		if c != "" {
			if _, ok := colors.Parse(c); !ok {
				return nil, errMsg("unsupported enclosure color: " + c)
			}
		}
	}
	n.Body = body
	if cmd == `\bbox` {
		n.Padding += n.Border / 2
	}
	return n, nil
}
func bboxLength(s string) (float64, error) {
	if len(s) < 3 {
		return 0, errMsg("invalid bbox length")
	}
	unit := s[len(s)-2:]
	v, err := strconv.ParseFloat(s[:len(s)-2], 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) || v < 0 || v > 100 {
		return 0, errMsg("invalid bbox length")
	}
	switch unit {
	case "px":
		v /= 16
	case "pt":
		v /= 10
	case "em":
	default:
		return 0, errMsg("unsupported bbox unit")
	}
	return v, nil
}
