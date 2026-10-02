package service

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/dslipak/pdf"
	"github.com/shopspring/decimal"

	"github.com/pircos/api/internal/domain"
)

// ParsedTransaction represents an individual operation extracted from a brokerage note.
type ParsedTransaction struct {
	OperationType    string          `json:"operation_type"` // "BUY" or "SELL"
	Ticker           string          `json:"ticker"`
	AssetDescription string          `json:"asset_description"`
	AssetClass       string          `json:"asset_class"`
	Quantity         decimal.Decimal `json:"quantity"`
	UnitPrice        decimal.Decimal `json:"unit_price"`
	TotalAmount      decimal.Decimal `json:"total_amount"`
	Costs            decimal.Decimal `json:"costs"`
	IsDayTrade       bool            `json:"is_day_trade"`
}

// ParsedBrokerageNote represents the complete structured output from parsing a Sinacor PDF note.
type ParsedBrokerageNote struct {
	NoteNumber     string              `json:"note_number"`
	BrokerName     string              `json:"broker_name"`
	OperationDate  time.Time           `json:"operation_date"`
	SettlementDate *time.Time          `json:"settlement_date,omitempty"`
	TotalCosts     decimal.Decimal     `json:"total_costs"`
	GrossAmount    decimal.Decimal     `json:"gross_amount"`
	NetAmount      decimal.Decimal     `json:"net_amount"`
	IRRF           decimal.Decimal     `json:"irrf"`
	Transactions   []ParsedTransaction `json:"transactions"`
	RawText        string              `json:"raw_text,omitempty"`
}

// SinacorParser parses Sinacor B3 standard brokerage notes.
type SinacorParser struct{}

// NewSinacorParser creates a new SinacorParser instance.
func NewSinacorParser() *SinacorParser {
	return &SinacorParser{}
}

// ParsePDF reads a PDF from an io.Reader and extracts the structured brokerage note.
func (p *SinacorParser) ParsePDF(r io.ReaderAt, size int64) (*ParsedBrokerageNote, error) {
	reader, err := pdf.NewReader(r, size)
	if err != nil {
		return nil, fmt.Errorf("reading pdf: %w", err)
	}

	var buf bytes.Buffer
	numPages := reader.NumPage()
	for i := 1; i <= numPages; i++ {
		page := reader.Page(i)
		text, err := page.GetPlainText(nil)
		if err != nil {
			continue
		}
		buf.WriteString(text)
		buf.WriteString("\n")
	}

	return p.ParseText(buf.String())
}

// Regular expressions for Sinacor parsing
var (
	// Date patterns
	reOperationDate = regexp.MustCompile(`(?i)(?:Data\s+(?:do\s+)?preg[aã]o|Data\s+Preg[aã]o)[\s:]*([0-3]?\d/[0-1]?\d/\d{4})`)
	reSettlementDate = regexp.MustCompile(`(?i)(?:L[ií]quido\s+para|Data\s+liquida[cç][aã]o)[\s:]*([0-3]?\d/[0-1]?\d/\d{4})`)
	reNoteNumber    = regexp.MustCompile(`(?i)(?:N[uú]m(?:ero)?\s+(?:da\s+)?nota|Nr\.\s*nota)[\s:]*(\d+)`)

	// Costs in the summary section
	reLiquidacao = regexp.MustCompile(`(?i)Taxa\s+de\s+liquida[cç][aã]o[\s:]*([0-9\.\,]+)`)
	reRegistro   = regexp.MustCompile(`(?i)Taxa\s+de\s+registro[\s:]*([0-9\.\,]+)`)
	reEmolumentos = regexp.MustCompile(`(?i)Emolumentos[\s:]*([0-9\.\,]+)`)
	reCorretagem = regexp.MustCompile(`(?i)(?:Taxa\s+de\s+corretagem|Corretagem)[\s:]*([0-9\.\,]+)`)
	reISS        = regexp.MustCompile(`(?i)ISS[\s:]*([0-9\.\,]+)`)
	reOutras     = regexp.MustCompile(`(?i)Outras\s+Bovespa[\s:]*([0-9\.\,]+)`)
	reIRRF       = regexp.MustCompile(`(?i)(?:I\.?R\.?R\.?F\.?|IRRF)(?:\s+(?:Day\s+Trade|opera[cç][õo]es))?[\s:]*([0-9\.\,]+)`)
	reTotalCustos = regexp.MustCompile(`(?i)(?:Total\s+custos\s*/\s*despesas|Total\s+despesas)[\s:]*([0-9\.\,]+)`)

	// Operation row regex:
	// Matches lines like:
	// 1-BOVESPA C VISTA PETR4 PETROBRAS PN 100 34,50 3.450,00 D
	// or:
	// C VISTA PETR4 100 34,50 3.450,00 D
	// or:
	// 1-BOVESPA V VISTA VALE3 VALE ON NM 200 65,00 13.000,00 C
	reOperationLine = regexp.MustCompile(`(?i)(?:1-BOVESPA\s+)?\b([CV])\s+(?:VISTA|FRACIONARIO|OPCAO|EXERCICIO)\s+(.+?)\s+(\d+[\.\d]*)\s+([0-9\.\,]+)\s+([0-9\.\,]+)\s+([CD])`)

	// Ticker pattern (e.g. PETR4, VALE3, HGLG11, BOVA11)
	reTicker = regexp.MustCompile(`\b([A-Z]{4}(?:3|4|5|6|11|34|33))\b`)
)

// ParseText parses plain text representation of a Sinacor brokerage note.
func (p *SinacorParser) ParseText(text string) (*ParsedBrokerageNote, error) {
	note := &ParsedBrokerageNote{
		RawText: text,
	}

	// 1. Extract dates
	if match := reOperationDate.FindStringSubmatch(text); len(match) > 1 {
		if t, err := time.Parse("02/01/2006", match[1]); err == nil {
			note.OperationDate = t
		}
	}
	if match := reSettlementDate.FindStringSubmatch(text); len(match) > 1 {
		if t, err := time.Parse("02/01/2006", match[1]); err == nil {
			note.SettlementDate = &t
		}
	}
	if match := reNoteNumber.FindStringSubmatch(text); len(match) > 1 {
		note.NoteNumber = match[1]
	}

	// If no settlement date was found, default to operation date + 2 business days
	if note.SettlementDate == nil && !note.OperationDate.IsZero() {
		defaultSettlement := note.OperationDate.AddDate(0, 0, 2)
		note.SettlementDate = &defaultSettlement
	}

	// 2. Extract costs
	var totalCosts decimal.Decimal
	costsFound := false

	// Try extracting explicit total costs first
	if match := reTotalCustos.FindStringSubmatch(text); len(match) > 1 {
		if val, err := parseBRDecimal(match[1]); err == nil && val.IsPositive() {
			totalCosts = val
			costsFound = true
		}
	}

	// Sum itemized costs if total was not directly found
	if !costsFound {
		costRegexes := []*regexp.Regexp{
			reLiquidacao, reRegistro, reEmolumentos, reCorretagem, reISS, reOutras,
		}
		for _, re := range costRegexes {
			if match := re.FindStringSubmatch(text); len(match) > 1 {
				if val, err := parseBRDecimal(match[1]); err == nil {
					totalCosts = totalCosts.Add(val)
				}
			}
		}
	}
	note.TotalCosts = totalCosts

	// Extract IRRF
	if match := reIRRF.FindStringSubmatch(text); len(match) > 1 {
		if val, err := parseBRDecimal(match[1]); err == nil {
			note.IRRF = val
		}
	}

	// 3. Extract transaction rows
	lines := strings.Split(text, "\n")
	var rawOperations []ParsedTransaction

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		match := reOperationLine.FindStringSubmatch(line)
		if len(match) == 7 {
			opTypeStr := strings.ToUpper(match[1])
			opType := domain.OpBuy
			if opTypeStr == "V" {
				opType = domain.OpSell
			}

			titleAndTicker := strings.TrimSpace(match[2])
			qtyStr := strings.ReplaceAll(match[3], ".", "")
			priceStr := match[4]
			totalStr := match[5]

			qty, err := decimal.NewFromString(qtyStr)
			if err != nil {
				continue
			}

			price, err := parseBRDecimal(priceStr)
			if err != nil {
				continue
			}

			total, err := parseBRDecimal(totalStr)
			if err != nil {
				total = qty.Mul(price)
			}

			// Extract ticker
			ticker := ""
			tickerMatch := reTicker.FindString(titleAndTicker)
			if tickerMatch != "" {
				ticker = tickerMatch
			} else {
				// Fallback: extract first word or uppercase token
				parts := strings.Fields(titleAndTicker)
				if len(parts) > 0 {
					ticker = parts[0]
				}
			}

			// Guess asset class
			assetClass := domain.AssetClassStocks
			if strings.HasSuffix(ticker, "11") {
				if strings.Contains(strings.ToUpper(titleAndTicker), "FII") ||
					strings.Contains(strings.ToUpper(titleAndTicker), "CI") {
					assetClass = domain.AssetClassFII
				} else {
					assetClass = domain.AssetClassETF
				}
			}

			// Check day trade indicator in line
			isDayTrade := strings.Contains(line, " D ") || strings.Contains(line, "\tD\t")

			rawOperations = append(rawOperations, ParsedTransaction{
				OperationType:    opType,
				Ticker:           ticker,
				AssetDescription: titleAndTicker,
				AssetClass:       assetClass,
				Quantity:         qty,
				UnitPrice:        price,
				TotalAmount:      total,
				Costs:            decimal.Zero, // will be prorated below
				IsDayTrade:       isDayTrade,
			})
		}
	}

	// 4. Prorate costs proportionally to financial volume
	var totalVolume decimal.Decimal
	for _, op := range rawOperations {
		totalVolume = totalVolume.Add(op.TotalAmount)
	}

	note.GrossAmount = totalVolume
	if totalVolume.IsPositive() && note.TotalCosts.IsPositive() {
		for i := range rawOperations {
			// Cost share = TotalCosts * (OpAmount / TotalVolume)
			share := note.TotalCosts.Mul(rawOperations[i].TotalAmount).Div(totalVolume).RoundBank(2)
			rawOperations[i].Costs = share
		}
	}

	note.Transactions = rawOperations
	note.NetAmount = totalVolume.Sub(note.TotalCosts)

	return note, nil
}

// parseBRDecimal parses a Brazilian formatted decimal number (e.g. "1.234,56" -> 1234.56).
func parseBRDecimal(val string) (decimal.Decimal, error) {
	val = strings.TrimSpace(val)
	val = strings.ReplaceAll(val, ".", "")
	val = strings.ReplaceAll(val, ",", ".")
	return decimal.NewFromString(val)
}
