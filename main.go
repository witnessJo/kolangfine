package main

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	kiwi "github.com/codingpot/kiwigo"
)

// preservePatterns lists regex patterns whose matches should be preserved as-is
// (not broken up by Kiwi's morphological analysis).
var preservePatterns = []*regexp.Regexp{
	// URLs
	regexp.MustCompile(`https?://\S+`),
	regexp.MustCompile(`www\.\S+`),
	// Email addresses
	regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`),
	// Korean date-time (e.g., 2024년 1월 15일 12시 30분 45초)
	regexp.MustCompile(`\d{4}년\s*\d{1,2}월\s*\d{1,2}일(?:\s*\d{1,2}시(?:\s*\d{1,2}분(?:\s*\d{1,2}초)?)?)?`),
	// Date-time with separator (e.g., 2024.01.15 12:30, 2024-01-15 12:30:00)
	regexp.MustCompile(`\d{4}[.\-/]\d{1,2}[.\-/]\d{1,2}\s+\d{1,2}:\d{2}(?::\d{2})?`),
	// Date with spaced dots (e.g., 2024. 01. 15 or 2024. 01. 15.)
	regexp.MustCompile(`\d{4}\.\s*\d{1,2}\.\s*\d{1,2}\.?`),
	// Date only (e.g., 2024.01.15, 2024-01-15, 2024/01/15)
	regexp.MustCompile(`\d{4}[.\-/]\d{1,2}[.\-/]\d{1,2}`),
	// Time with colon (e.g., 12:30, 12:30:00)
	regexp.MustCompile(`\d{1,2}:\d{2}(?::\d{2})?`),
	// File paths (Windows)
	regexp.MustCompile(`[A-Z]:\\[\w\\.\-]+`),
	// File paths (Unix)
	regexp.MustCompile(`/(?:[\w.\-]+/)+[\w.\-]+`),
	// Full-width parenthesized expressions (e.g., （주）, （株）, （별첨）)
	regexp.MustCompile(`（[^）]+）`),
	// Signed numbers with optional decimal and unit (e.g., -5.3%, +12.5%, ++5.3%)
	regexp.MustCompile(`[+\-]{1,2}\d+(?:\.\d+)?%?`),
	// English words/abbreviations (2+ chars to avoid single-letter Korean jamo conflicts)
	regexp.MustCompile(`[A-Za-z][A-Za-z0-9]{1,}(?:\.[A-Za-z0-9]+)*`),
}

// Tags that should NOT have a space before them (they attach to the preceding word)
var noSpaceTags = map[kiwi.POSType]bool{
	// Particles (조사)
	kiwi.POS_JKS: true, kiwi.POS_JKC: true, kiwi.POS_JKG: true,
	kiwi.POS_JKO: true, kiwi.POS_JKB: true, kiwi.POS_JKV: true,
	kiwi.POS_JKQ: true, kiwi.POS_JX: true, kiwi.POS_JC: true,
	// Endings (어미)
	kiwi.POS_EP: true, kiwi.POS_EF: true, kiwi.POS_EC: true,
	kiwi.POS_ETN: true, kiwi.POS_ETM: true,
	// Suffixes (접미사)
	kiwi.POS_XSN: true, kiwi.POS_XSV: true, kiwi.POS_XSA: true,
	// Copula
	kiwi.POS_VCP: true, kiwi.POS_VCN: true,
	// Punctuation
	kiwi.POS_SF: true, kiwi.POS_SP: true, kiwi.POS_SS: true,
	kiwi.POS_SSO: true, kiwi.POS_SSC: true, kiwi.POS_SE: true, kiwi.POS_SO: true,
}

// runeSlice returns runes[start:end] as a string, handling bounds safely.
func runeSlice(runes []rune, start, end int) string {
	if start < 0 {
		start = 0
	}
	if end > len(runes) {
		end = len(runes)
	}
	if start >= end {
		return ""
	}
	return string(runes[start:end])
}

// correctSpacing takes a string with potentially incorrect spacing and returns
// a properly spaced string using Kiwi's morphological analysis.
func correctSpacing(k *kiwi.Kiwi, input string) (string, error) {
	// Preserve patterns (URLs, emails, paths, English words) with PUA placeholders
	var preserved []string
	text := input
	for _, pat := range preservePatterns {
		text = pat.ReplaceAllStringFunc(text, func(match string) string {
			placeholder := string(rune(0xE000 + len(preserved)))
			preserved = append(preserved, match)
			return placeholder
		})
	}

	// Remove spaces, but keep a space before/after PUA placeholders so Kiwi
	// treats them as separate tokens.
	var stripBuf strings.Builder
	for _, r := range text {
		if unicode.IsSpace(r) {
			continue
		}
		// Insert space before a PUA placeholder if the previous char wasn't a space
		if r >= 0xE000 && r <= 0xE000+rune(len(preserved)-1) {
			if stripBuf.Len() > 0 {
				stripBuf.WriteRune(' ')
			}
			stripBuf.WriteRune(r)
			continue
		}
		stripBuf.WriteRune(r)
	}
	stripped := strings.TrimSpace(stripBuf.String())
	if stripped == "" {
		return "", nil
	}

	results, err := k.Analyze(stripped, 1, kiwi.KIWI_MATCH_ALL)
	if err != nil {
		return "", err
	}
	if len(results) == 0 || len(results[0].Tokens) == 0 {
		return input, nil
	}

	tokens := results[0].Tokens
	runes := []rune(stripped)
	totalRunes := len(runes)

	// Build word boundaries using Position field.
	// Position is the rune index in the original (stripped) text where the token starts.
	// We extract text between token positions from the original string to preserve
	// the actual characters (Kiwi's Form may normalize characters).
	var sb strings.Builder

	for i, tok := range tokens {
		// Determine the end position of this token's span in the original text
		var endPos int
		if i+1 < len(tokens) {
			endPos = tokens[i+1].Position
		} else {
			endPos = totalRunes
		}

		// Extract the original text for this token
		segment := runeSlice(runes, tok.Position, endPos)

		if i == 0 {
			sb.WriteString(segment)
			continue
		}

		needSpace := !noSpaceTags[tok.Tag]

		// After opening parenthesis, no space
		if tokens[i-1].Tag == kiwi.POS_SSO {
			needSpace = false
		}

		// After prefix (제~), no space
		if tokens[i-1].Tag == kiwi.POS_XPN {
			needSpace = false
		}
		// "제" as MM before number (제9조, 제21항) — Kiwi sometimes tags 제 as MM instead of XPN
		if tokens[i-1].Tag == kiwi.POS_MM && tokens[i-1].Form == "제" && tok.Tag == kiwi.POS_SN {
			needSpace = false
		}

		// SW (special chars like ･, /, %) attaches to adjacent tokens
		if tok.Tag == kiwi.POS_SW || tokens[i-1].Tag == kiwi.POS_SW {
			needSpace = false
		}

		// Adjacent numbers (e.g., Kiwi splitting "100,000" into parts)
		if tok.Tag == kiwi.POS_SN && tokens[i-1].Tag == kiwi.POS_SN {
			needSpace = false
		}

		// Counter/unit nouns after numbers: no space (e.g., 110조, 21항, 100,000좌)
		if tokens[i-1].Tag == kiwi.POS_SN && (tok.Tag == kiwi.POS_NR || tok.Tag == kiwi.POS_NNB) {
			needSpace = false
		}
		// Single-character NNG counters after numbers (조, 항, 호, etc.)
		if tokens[i-1].Tag == kiwi.POS_SN && tok.Tag == kiwi.POS_NNG && utf8.RuneCountInString(tok.Form) == 1 {
			needSpace = false
		}

		if needSpace {
			sb.WriteString(" ")
		}
		sb.WriteString(segment)
	}

	result := sb.String()
	for i, u := range preserved {
		placeholder := string(rune(0xE000 + i))
		result = strings.ReplaceAll(result, placeholder, u)
	}
	return result, nil
}

// correctSpacingArray takes a string array (tokens with potentially bad spacing),
// joins them, corrects spacing, then re-splits into properly spaced tokens.
func correctSpacingArray(k *kiwi.Kiwi, input []string) ([]string, error) {
	joined := strings.Join(input, "")
	corrected, err := correctSpacing(k, joined)
	if err != nil {
		return nil, err
	}
	return strings.Fields(corrected), nil
}

func buildKiwi() *kiwi.Kiwi {
	kb := kiwi.NewBuilder("./base", 1, kiwi.KIWI_BUILD_INTEGRATE_ALLOMORPH)

	// Add compound nouns / domain-specific terms that Kiwi might split incorrectly
	compoundNouns := []string{
		"투자신탁", "투자증권", "집합투자기구", "집합투자증권",
		"상장지수집합투자기구", "수익증권", "신탁계약기간",
		"자금납입", "환매청구접수", "투자대상", "가격변동",
		"이자율변동", "거시경제", "운용전문인력", "성과보수",
		"기타비용", "순자산", "연평잔액", "고위험자산",
		"수익률", "투자수익", "신탁회사", "수익자",
		"정배수", "기준일", "총보수", "차감전",
	}
	for _, w := range compoundNouns {
		kb.AddWord(w, kiwi.POS_NNP, 10)
	}

	// Add words that Kiwi mis-parses
	kb.AddWord("당해", kiwi.POS_MM, 10) // determiner: 당해 (this/that)

	k := kb.Build()
	return k
}

func main() {
	k := buildKiwi()
	defer k.Close()

	fmt.Println("=== String Examples ===")

	stringExamples := []struct {
		input string
	}{
		{"다음 각 호와 관련된 법 령 및 규정이 변경되는 경우에는 그 변경된 바에 따릅니다."},
		{"투자대상 의 규모 및 유동성 등을 고려하여 투자하고 있으며, 사후적으로 투자대상의 특성에 맞게 유동성 위험과 관 련한 내부 관리"},
		{"투자자는 수익증권 입고 시점을 사 전에 점검하여 주시기 바랍니다."},
		{"수 익자가 당해 환매청구접수의 취소를 하지 아니하였을 경우"},
		{"고위험자산에 8 0%이상 투자하는 집합투자기구"},
		{"기준일 현재 동 운용전문인력이 운용 중인 성과보수가 약정된 집합투자기구는 없습니다."},
		{"수익자는 100,000좌 또는 그 정배수(200,000좌, 300,000좌 등)로만 설정을 청구하거나"},
		{"총보수･비용 비율은 이 투자신탁에서 지출되는 보수와 기타비용 총액을 순자산 연평잔액(보수 및 비 용 차감전 기준)으로 나누어 산출합니다."},
		{"법 제110조에 의하여 신탁회사가 발행한 수익증권, 법 제9조 제21항의 규정에 의한 집합투자증권 및 법 제234조의 규정에 의한 상장지수집합투자기구 집합투자증권(이와 유사한 것으로 서 외국 법령에 따라 발행된 것을 포함)"},
		{"www.example.com은 당해 서비스와 관련하여 필요하다고 인정되는 경우에는 언제든지 그 내용을 변경할 수 있습니다."},
		{"-5.3%하락하였다.++5.3%상승하였다."},
		// 날짜/시간 보존 테스트
		{"2024.01.15 12:30에 회 의가 시작됩니다."},
		{"2024-01-15 12:30:00 기준으로 산출합니다."},
		{"2024년 1월 15일 12시 30분에 개최합니다."},
		{"2024. 01. 15. 현재 기준입니다."},
		{"2024/01/15부터 적용됩니다."},
		{"매일 14:30에 기준가격이 산출됩니다."},
		// 파일 경로 보존 테스트
		{"설정 파일은 C:\\Users\\admin\\config.ini에 저장됩니다."},
		{"로그 파일은 /var/log/app/error.log에 기록됩니다."},
		// 전각 괄호 보존 테스트
		{"（주）회사명은 투자신탁을 운용합니다."},
		{"（별첨）참조하시기 바랍니다."},
		// 탭/개행 처리 테스트
		{"투자자는\t수익증권\t입고 시점을 점검하여 주시기 바랍니다."},
		{"투자대상 의 규모\n및 유동성 등을 고려하여\n투자하고 있습니다."},
		{"수익자는\t\t100,000좌 또는\t그 정배수로 청구합니다."},
		// 괄호 위치 보존 테스트
		{"연평잔액(보수 및 비 용 차감전 기준)으로 나누어 산출합니다."},
		{"정배수(200,000좌, 300,000좌 등)로만 설정을 청구합니다."},
		{"집합투자증권(이와 유사한 것으로 서 외국 법령에 따른 것을 포함)"},
	}

	for _, ex := range stringExamples {
		result, err := correctSpacing(k, ex.input)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}
		fmt.Printf("Input:  %s\n", ex.input)
		fmt.Printf("Output: %s\n\n", result)
	}

	fmt.Println("=== Array Examples ===")

	arrayExamples := [][]string{
		{"이", "투자신탁은", "추가", "자금납입이", "가능한", "투", "자신탁으로", "별도의", "신탁계약기간을", "정하지", "않고", "있습니다."},
		{"투자대상", "의", "규모", "및", "유동성", "등을", "고려하여", "투자하고", "있으며,", "사후적으로", "투자대상의", "특성에", "맞게", "유동성", "위험과", "관", "련한", "내부", "관리"},
		{"수", "익자가", "당해", "환매청구접수의", "취소를", "하지", "아니하였을", "경우"},
		{"고위험자산에", "8", "0%이상", "투자하는", "집합투자기구"},
	}

	for _, ex := range arrayExamples {
		result, err := correctSpacingArray(k, ex)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}
		fmt.Printf("Input:  %v\n", ex)
		fmt.Printf("Output: %v\n\n", result)
	}
}
