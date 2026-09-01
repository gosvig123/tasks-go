package task

import "strings"

const descriptionTagPrefix = `@desc:"`

func extractDescription(line string) (string, string, bool) {
	start := strings.Index(line, descriptionTagPrefix)
	if start < 0 {
		return "", line, false
	}
	value, consumed, ok := decodeDescription(line[start+len(descriptionTagPrefix):])
	if !ok {
		return "", line, false
	}
	end := start + len(descriptionTagPrefix) + consumed
	remainder := strings.TrimSpace(line[:start] + " " + line[end:])
	return value, remainder, true
}

func decodeDescription(input string) (string, int, bool) {
	var value strings.Builder
	for index := 0; index < len(input); index++ {
		if input[index] == '"' {
			return value.String(), index + 1, true
		}
		if input[index] != '\\' {
			value.WriteByte(input[index])
			continue
		}
		decoded, consumed := decodeDescriptionEscape(input[index+1:])
		value.WriteString(decoded)
		index += consumed
	}
	return "", 0, false
}

func decodeDescriptionEscape(input string) (string, int) {
	if strings.HasPrefix(input, `r\n`) {
		return "\n", 3
	}
	if input == "" {
		return `\`, 0
	}
	switch input[0] {
	case 'n', 'r':
		return "\n", 1
	case '\\':
		return `\`, 1
	case '"':
		return `"`, 1
	default:
		return `\` + input[:1], 1
	}
}

func encodeDescription(value string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	return strings.NewReplacer(
		`\`, `\\`, `"`, `\"`, "\n", `\n`,
	).Replace(value)
}
