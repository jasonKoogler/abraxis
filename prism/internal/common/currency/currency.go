package currency

import (
	"errors"
	"fmt"
	"strings"
)

type Currency string

const (
	CurrencyUSD Currency = "USD"
	CurrencyEUR Currency = "EUR"
	CurrencyGBP Currency = "GBP"
	CurrencyCAD Currency = "CAD"
	CurrencyAUD Currency = "AUD"
	CurrencyJPY Currency = "JPY"
	CurrencyCHF Currency = "CHF"
	CurrencySEK Currency = "SEK"
	CurrencyNOK Currency = "NOK"
	CurrencyNZD Currency = "NZD"
)

func (c Currency) String() string {
	return string(c)
}

func (c Currency) IsValid() bool {
	return IsValidCurrency(c.String())
}

func IsValidCurrency(c string) bool {
	if len(c) != 3 {
		return false
	}
	switch Currency(c) {
	case CurrencyUSD,
		CurrencyEUR,
		CurrencyGBP,
		CurrencyCAD,
		CurrencyAUD,
		CurrencyJPY,
		CurrencyCHF,
		CurrencySEK,
		CurrencyNOK,
		CurrencyNZD:
		return true
	default:
		return false
	}
}

var CurrencyValues = []any{
	CurrencyUSD,
	CurrencyEUR,
	CurrencyGBP,
	CurrencyCAD,
	CurrencyAUD,
	CurrencyJPY,
	CurrencyCHF,
	CurrencySEK,
	CurrencyNOK,
	CurrencyNZD,
}

func (c Currency) Symbol() string {
	switch c {
	case CurrencyUSD:
		return "$"
	case CurrencyEUR:
		return "€"
	case CurrencyGBP:
		return "£"
	case CurrencyCAD:
		return "CA$"
	case CurrencyAUD:
		return "AU$"
	case CurrencyJPY:
		return "¥"
	case CurrencyCHF:
		return "CHF"
	case CurrencySEK:
		return "kr"
	case CurrencyNOK:
		return "kr"
	case CurrencyNZD:
		return "NZ$"
	default:
		return ""
	}
}

func (c Currency) Format(amount int64) string {
	return fmt.Sprintf("%s %d", c.Symbol(), amount)
}

func MakeCurrency(currency string) (*Currency, error) {
	if !IsValidCurrency(strings.ToUpper(currency)) {
		return nil, ErrInvalidCurrency
	}
	cur := Currency(strings.ToUpper(currency))
	return &cur, nil
}

var ErrInvalidCurrency = errors.New("invalid currency")
