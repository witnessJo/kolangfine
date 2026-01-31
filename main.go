package main

import (
	"fmt"
	"strings"
	"unicode/utf8"

	kiwi "github.com/codingpot/kiwigo"
)

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
	stripped := strings.ReplaceAll(input, " ", "")
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

	return sb.String(), nil
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
