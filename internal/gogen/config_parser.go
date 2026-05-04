package gogen

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

type configParser struct {
	src string
	pos int
}

func ParseConfig(src string) (Config, error) {
	p := &configParser{src: src}
	var cfg Config
	for {
		p.skipSpaceAndComments()
		if p.eof() {
			return cfg, nil
		}
		keyword, err := p.readIdent()
		if err != nil {
			return Config{}, err
		}
		switch keyword {
		case "model":
			model, err := p.readModel()
			if err != nil {
				return Config{}, err
			}
			cfg.Models = append(cfg.Models, model)
		case "size":
			item, err := p.readNamedString()
			if err != nil {
				return Config{}, err
			}
			cfg.Sizes = append(cfg.Sizes, SizeAlias{Name: item.name, Value: item.value})
		default:
			return Config{}, p.errf("unexpected top-level block %q", keyword)
		}
	}
}

func (p *configParser) readModel() (ModelConfig, error) {
	p.skipSpaceAndComments()
	name, err := p.readString()
	if err != nil {
		return ModelConfig{}, err
	}
	model := ModelConfig{Name: name, Params: map[string]any{}, Headers: map[string]string{}}
	p.skipSpaceAndComments()
	if err := p.expect('{'); err != nil {
		return ModelConfig{}, err
	}
	for {
		p.skipSpaceAndComments()
		if p.peek() == '}' {
			p.pos++
			return model, nil
		}
		key, err := p.readIdent()
		if err != nil {
			return ModelConfig{}, err
		}
		p.skipSpaceAndComments()
		if err := p.expect('='); err != nil {
			return ModelConfig{}, err
		}
		p.skipSpaceAndComments()
		value, err := p.readValue()
		if err != nil {
			return ModelConfig{}, err
		}
		if err := assignModelValue(&model, key, value); err != nil {
			return ModelConfig{}, err
		}
	}
}

func assignModelValue(model *ModelConfig, key string, value any) error {
	switch key {
	case "name":
		v, ok := value.(string)
		if !ok {
			return fmt.Errorf("name must be a string")
		}
		model.Name = v
	case "model_name":
		v, ok := value.(string)
		if !ok {
			return fmt.Errorf("model_name must be a string")
		}
		model.ModelName = v
	case "base_url":
		v, ok := value.(string)
		if !ok {
			return fmt.Errorf("base_url must be a string")
		}
		model.BaseURL = v
	case "api_key":
		v, ok := value.(string)
		if !ok {
			return fmt.Errorf("api_key must be a string")
		}
		model.APIKey = v
	case "project":
		v, ok := value.(string)
		if !ok {
			return fmt.Errorf("project must be a string")
		}
		model.Project = v
	case "default":
		v, ok := value.(bool)
		if !ok {
			return fmt.Errorf("default must be a boolean")
		}
		model.Default = v
	case "params":
		v, ok := value.(map[string]any)
		if !ok {
			return fmt.Errorf("params must be a JSON object")
		}
		model.Params = v
	case "headers":
		v, ok := value.(map[string]any)
		if !ok {
			return fmt.Errorf("headers must be a JSON object")
		}
		model.Headers = map[string]string{}
		for hk, hv := range v {
			s, ok := hv.(string)
			if !ok {
				return fmt.Errorf("header %s must be a string", hk)
			}
			model.Headers[hk] = s
		}
	case "timeout":
		v, ok := numberToFloat(value)
		if !ok {
			return fmt.Errorf("timeout must be a number")
		}
		model.Timeout = &v
	case "max_retries":
		v, ok := numberToFloat(value)
		if !ok {
			return fmt.Errorf("max_retries must be a number")
		}
		i := int(v)
		model.MaxRetries = &i
	default:
		if model.Params == nil {
			model.Params = map[string]any{}
		}
		model.Params[key] = value
	}
	return nil
}

type namedString struct {
	name  string
	value string
}

func (p *configParser) readNamedString() (namedString, error) {
	p.skipSpaceAndComments()
	name, err := p.readString()
	if err != nil {
		return namedString{}, err
	}
	p.skipSpaceAndComments()
	if err := p.expect('='); err != nil {
		return namedString{}, err
	}
	p.skipSpaceAndComments()
	value, err := p.readString()
	if err != nil {
		return namedString{}, err
	}
	return namedString{name: name, value: value}, nil
}

func (p *configParser) readValue() (any, error) {
	p.skipSpaceAndComments()
	if strings.HasPrefix(p.src[p.pos:], `"""`) {
		return p.readTripleString()
	}
	switch p.peek() {
	case '"':
		return p.readString()
	case '{', '[':
		return p.readJSONValue()
	default:
		token := p.readBareToken()
		switch token {
		case "":
			return nil, p.errf("expected value")
		case "true":
			return true, nil
		case "false":
			return false, nil
		}
		if strings.ContainsAny(token, ".eE") {
			v, err := strconv.ParseFloat(token, 64)
			if err == nil {
				return v, nil
			}
		} else {
			v, err := strconv.Atoi(token)
			if err == nil {
				return float64(v), nil
			}
		}
		return nil, p.errf("unquoted value %q is not supported", token)
	}
}

func (p *configParser) readString() (string, error) {
	if strings.HasPrefix(p.src[p.pos:], `"""`) {
		return p.readTripleString()
	}
	if p.peek() != '"' {
		return "", p.errf("expected quoted string")
	}
	start := p.pos
	p.pos++
	escaped := false
	for !p.eof() {
		ch := p.src[p.pos]
		p.pos++
		if escaped {
			escaped = false
			continue
		}
		if ch == '\\' {
			escaped = true
			continue
		}
		if ch == '"' {
			return strconv.Unquote(p.src[start:p.pos])
		}
	}
	return "", p.errf("unterminated string")
}

func (p *configParser) readTripleString() (string, error) {
	if !strings.HasPrefix(p.src[p.pos:], `"""`) {
		return "", p.errf("expected triple-quoted string")
	}
	p.pos += 3
	end := strings.Index(p.src[p.pos:], `"""`)
	if end < 0 {
		return "", p.errf("unterminated triple-quoted string")
	}
	value := p.src[p.pos : p.pos+end]
	p.pos += end + 3
	if strings.HasPrefix(value, "\r\n") {
		value = value[2:]
	} else if strings.HasPrefix(value, "\n") {
		value = value[1:]
	}
	if strings.HasSuffix(value, "\r\n") {
		value = value[:len(value)-2]
	} else if strings.HasSuffix(value, "\n") {
		value = value[:len(value)-1]
	}
	return value, nil
}

func (p *configParser) readJSONValue() (any, error) {
	start := p.pos
	open := p.peek()
	close := byte('}')
	if open == '[' {
		close = ']'
	}
	depth := 0
	inString := false
	escaped := false
	for !p.eof() {
		ch := p.src[p.pos]
		p.pos++
		if inString {
			if escaped {
				escaped = false
			} else if ch == '\\' {
				escaped = true
			} else if ch == '"' {
				inString = false
			}
			continue
		}
		switch ch {
		case '"':
			inString = true
		case open:
			depth++
		case close:
			depth--
			if depth == 0 {
				raw := p.src[start:p.pos]
				var value any
				if err := json.Unmarshal([]byte(raw), &value); err != nil {
					return nil, p.errf("invalid JSON value: %v", err)
				}
				return value, nil
			}
		case '{':
			if open != '{' {
				depth++
			}
		case '[':
			if open != '[' {
				depth++
			}
		case '}':
			if open != '{' {
				depth--
			}
		case ']':
			if open != '[' {
				depth--
			}
		}
	}
	return nil, p.errf("unterminated JSON value")
}

func (p *configParser) readIdent() (string, error) {
	p.skipSpaceAndComments()
	start := p.pos
	for !p.eof() {
		ch := rune(p.src[p.pos])
		if unicode.IsLetter(ch) || unicode.IsDigit(ch) || ch == '_' || ch == '-' {
			p.pos++
			continue
		}
		break
	}
	if start == p.pos {
		return "", p.errf("expected identifier")
	}
	return p.src[start:p.pos], nil
}

func (p *configParser) readBareToken() string {
	start := p.pos
	for !p.eof() {
		ch := p.peek()
		if unicode.IsSpace(rune(ch)) || ch == '}' || ch == '#' {
			break
		}
		if ch == '/' && p.pos+1 < len(p.src) && p.src[p.pos+1] == '/' {
			break
		}
		p.pos++
	}
	return p.src[start:p.pos]
}

func (p *configParser) skipSpaceAndComments() {
	for !p.eof() {
		ch := p.peek()
		if unicode.IsSpace(rune(ch)) {
			p.pos++
			continue
		}
		if ch == '#' {
			p.skipLine()
			continue
		}
		if ch == '/' && p.pos+1 < len(p.src) && p.src[p.pos+1] == '/' {
			p.skipLine()
			continue
		}
		return
	}
}

func (p *configParser) skipLine() {
	for !p.eof() && p.peek() != '\n' {
		p.pos++
	}
}

func (p *configParser) expect(ch byte) error {
	if p.eof() || p.peek() != ch {
		return p.errf("expected %q", ch)
	}
	p.pos++
	return nil
}

func (p *configParser) peek() byte {
	if p.eof() {
		return 0
	}
	return p.src[p.pos]
}

func (p *configParser) eof() bool {
	return p.pos >= len(p.src)
}

func (p *configParser) errf(format string, args ...any) error {
	line, col := 1, 1
	for i := 0; i < p.pos && i < len(p.src); i++ {
		if p.src[i] == '\n' {
			line++
			col = 1
		} else {
			col++
		}
	}
	return fmt.Errorf("line %d, column %d: %s", line, col, fmt.Sprintf(format, args...))
}

func numberToFloat(value any) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case int:
		return float64(v), true
	default:
		return 0, false
	}
}
