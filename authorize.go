package AuthorizeNet

// Authorize.Net support

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/pschlump/Go-FTL/server/lib"
)

type AuthorizeNetType struct {
	Amount          string `authorizeNet:"x_amount"`
	BillingAddress  string `authorizeNet:"x_address"`
	BillingCity     string `authorizeNet:"x_city"`
	BillingCountry  string `authorizeNet:"x_country"`
	BillingState    string `authorizeNet:"x_state"`
	BillingZip      string `authorizeNet:"x_zip"`
	Company         string `authorizeNet:"x_company"`
	CustomerId      string `authorizeNet:"x_cust_id"`
	CustomerIp      string `authorizeNet:"x_customer_ip"`
	Description     string `authorizeNet:"x_description"`
	Email           string `authorizeNet:"x_email"`
	FirstName       string `authorizeNet:"x_first_name"`
	InvoiceNumber   string `authorizeNet:"x_invoice_num"`
	LastName        string `authorizeNet:"x_last_name"`
	Phone           string `authorizeNet:"x_phone"`
	ShipToAddress   string `authorizeNet:"x_ship_to_address"`
	ShipToCity      string `authorizeNet:"x_ship_to_city"`
	ShipToCompany   string `authorizeNet:"x_ship_to_company"`
	ShipToCountry   string `authorizeNet:"x_ship_to_country"`
	ShipToFirstName string `authorizeNet:"x_ship_to_first_name"`
	ShipToLastName  string `authorizeNet:"x_ship_to_last_name"`
	ShipToState     string `authorizeNet:"x_ship_to_state"`
	ShipToZip       string `authorizeNet:"x_ship_to_zip"`
}

type AuthorizeResponse struct {
	Amount     string `json:"Amount"`
	AuthCode   string `json:"AuthorizationCode"`
	AvsResp    string `json:"AvsResponse"`
	CVVResp    string `json:"CCVResponse"`
	RawData    string `json:"RawResponse"`
	ReasonCode string `json:"ReasonCode"`
	ReasonText string `json:"ReasonText"`
	RespCode   string `json:"ResponseCode"`
	IsApproved bool   `json:"IsApproved"`
	Tax        string `json:"Tax"`
	TransId    string `json:"TransactionId"`
	TransMd5   string `json:"TransactionMD5"`
	TransType  string `json:"TransactionType"`
}

const (
	METHOD_AMEX       = "amex"
	METHOD_DISCOVER   = "discover"
	METHOD_MASTERCARD = "mastercard"
	METHOD_VISA       = "visa"
)

type CardInfoType struct {
	CreditCardNumber string `authorizeNet:"x_card_num"`
	CVV              string `authorizeNet:"x_card_code"`
	Month_Year       string `authorizeNet:"x_exp_date"`
	Method           string `authorizeNet:"x_method"`
}

var ErrInvalidCC = errors.New("Invalid Credit Card Nubmer.")
var ErrExpiredCC = errors.New("Expired credit card.")
var ErrInvalidCCV = errors.New("Invalid CVV.")
var ErrUnsupportedCC = errors.New("Unsupported Credit Card Type")

// Add the AuthorizeNetType values to the given url.Values map for authorize.net
func (a AuthorizeNetType) AddToUrlValues(urlValues url.Values) {
	st := reflect.TypeOf(a)
	sv := reflect.ValueOf(a)
	for i := 0; i < st.NumField(); i++ {
		field := st.Field(i)
		x_name := field.Tag.Get("authorizeNet")
		val := sv.FieldByIndex([]int{i}).String()
		if val != "" {
			// fmt.Printf("authorizeNet=%s value=%s\n", x_name, val)
			urlValues[x_name] = []string{val}
		}
	}
}

type AuthorizeNet struct {
	AuthorizeNetProvider string // something like: "https://secure.authorize.net/gateway/transact.dll" -- where we will POST to
	Login                string // authorize.net login
	Key                  string // authorize.net key
	DupWindow            int    // duplicate window	-- prevent duplicates via a time window in seconds, default 120 sec
	TestMode             bool   // test mode or not
}

func NewAuthorizeNet(Provider, login, key string, testMode bool, dupWindow int) (rv AuthorizeNet) {
	if Provider == "" {
		Provider = "https://secure.authorize.net/gateway/transact.dll"
	}
	if dupWindow == -1 {
		dupWindow = 120
	}
	return AuthorizeNet{
		AuthorizeNetProvider: Provider,
		Login:                login,
		Key:                  key,
		DupWindow:            dupWindow,
		TestMode:             testMode,
	}
}

func (a AuthorizeNet) initData(ty string, ex ...string) (rv url.Values) {
	rv = url.Values{
		"x_type":       {ty},
		"x_login":      {a.Login},
		"x_tran_key":   {a.Key},
		"x_method":     {"CC"},
		"x_version":    {"3.1"},
		"x_delim_data": {"TRUE"},
		"x_delim_char": {"|"},
		"x_encap_char": {`"`},
	}
	for i := 0; i < len(ex); i += 2 {
		if i+1 < len(ex) {
			rv[ex[i]] = []string{ex[i+1]}
		} else {
			fmt.Printf("Need even number of parameters in list\n")
		}
	}
	return
}

// Authorize a transaction (does not charge) returns a response
func (a AuthorizeNet) Authorize(card CardInfoType, data AuthorizeNetType, emailCustomer bool) AuthorizeResponse {
	vals := a.initData("AUTH_ONLY", "x_relay_response", "FALSE", "x_duplicate_window", fmt.Sprintf("%d", a.DupWindow), "x_email_customer", Bool(emailCustomer).UpperString())
	card.AddToUrlValues(vals)
	data.AddToUrlValues(vals)
	vals.Set("x_test_request", Bool(a.TestMode).UpperString())
	return a.Post(vals)
}

// Captures a previously authorized card An Empty amount string means full amount
func (a AuthorizeNet) CapturePreauth(transactionId string, amount string) AuthorizeResponse {
	data := a.initData("PRIOR_AUTH_CAPTURE", "x_trans_id", transactionId)
	if amount != "" {
		data["x_amount"] = []string{amount}
	}
	data.Set("x_test_request", Bool(a.TestMode).UpperString())
	return a.Post(data)
}

// Post: posts a query to atuhorize.net and returns the response
func (a AuthorizeNet) Post(data url.Values) (rv AuthorizeResponse) {
	resp, err := http.PostForm(a.AuthorizeNetProvider, data)
	defer resp.Body.Close()
	if err != nil {
		rv.ReasonText = "Failed to connect to payment gateway." // xyzzy - should be an error
		return
	}

	if resp.StatusCode != http.StatusOK {
		logrus.Errorf("Authorize Provider Status Code %d", resp.StatusCode)
		rv.ReasonText = "Payment gateway returned an error."
		return
	}
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logrus.Errorf("Authorize Provider - Unable to read body: %s", err)
		rv.ReasonText = "Payment gateway invalid body."
		return
	}
	if db1 {
		fmt.Printf("Response Body: %s\n", bodyBytes)
	}
	body := string(bodyBytes)
	rv.RawData = body

	ss := strings.Split(body, "|")
	for ii, vv := range ss {
		if len(vv) > 1 && vv[0:1] == `"` && vv[len(vv)-1:] == `"` {
			ss[ii] = vv[1 : len(vv)-1]
		}
	}

	return AuthorizeResponse{
		RespCode:   ss[0],
		IsApproved: ss[0] == "1",
		ReasonCode: ss[2],
		ReasonText: ss[3],
		AuthCode:   ss[4],
		AvsResp:    ss[5],
		TransId:    ss[6],
		Amount:     ss[9],
		TransType:  ss[11],
		Tax:        ss[32],
		TransMd5:   ss[37],
		CVVResp:    ss[38],
		RawData:    body,
	}

}

// Convert response to JSON for sort-of- human readable output
func (r *AuthorizeResponse) String() string {
	return lib.SVar(r)
}

// --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------
// card
// --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------

// Reinitialize the credit card structure
func (c CardInfoType) Wipe() {
	c.CVV, c.CreditCardNumber, c.Month_Year, c.Method = "", "", "", ""
}

// ValidateCard checks that the card data appears to be valid (not expired, right number length etc...)
// No back authorization occurs.  Lhun is checked.
func (c CardInfoType) ValidateCard() error {
	s2 := strings.Split(c.Month_Year, "/")
	if len(s2) != 2 {
		logrus.Errorf("Expired - invalid month/year - not supplied")
		return ErrExpiredCC
	}
	m, err := strconv.Atoi(s2[0])
	if err != nil {
		logrus.Errorf("Expired - invalid date - not a number - month")
		return ErrExpiredCC
	}
	y, err := strconv.Atoi(s2[1])
	if err != nil {
		logrus.Errorf("Expired - invalid date - not a number - year")
		return ErrExpiredCC
	}
	if y < time.Now().Year() {
		logrus.Errorf("Expired - past expiration date")
		return ErrExpiredCC
	}
	if y == time.Now().Year() && m <= int(time.Now().Month()) {
		logrus.Errorf("Expired - past expiration month")
		return ErrExpiredCC
	}
	if c.CreditCardNumber == "5555555555" {
		logrus.Errorf("Test data for card number")
		return nil // test credit card -> pass trough
	}
	if len(c.CVV) < 3 || len(c.CVV) > 4 {
		logrus.Errorf("CCV is incorrect length")
		return ErrInvalidCCV
	}
	l := len(c.CreditCardNumber)
	if l < 13 { // always at east 13 digits
		logrus.Errorf("CC is too shrot")
		return ErrInvalidCC
	}
	c.Method = strings.ToLower(c.Method)
	switch c.Method {
	case METHOD_VISA:
		if !strings.HasPrefix(c.CreditCardNumber, "4") || l != 16 && l != 13 {
			logrus.Errorf("Visa not start with '4'")
			return ErrInvalidCC
		}
	case METHOD_MASTERCARD:
		if !(strings.HasPrefix(c.CreditCardNumber, "5")) || l != 16 {
			logrus.Errorf("MC not start with '5'")
			return ErrInvalidCC
		}
	case METHOD_AMEX:
		if !(strings.HasPrefix(c.CreditCardNumber, "34") || strings.HasPrefix(c.CreditCardNumber, "37")) || l != 15 {
			logrus.Errorf("Amex not start with '34'")
			return ErrInvalidCC
		}
	case METHOD_DISCOVER:
		if !strings.HasPrefix(c.CreditCardNumber, "6011") || l != 16 {
			logrus.Errorf("Discover not start with '6011'")
			return ErrInvalidCC
		}
	default:
		logrus.Errorf("Invalid credit card type of %s", c.Mehotd)
		return ErrUnsupportedCC
	}
	// Check the card checksum: - http://en.wikipedia.org/wiki/Luhn_algorithm
	if CheckCCLhun(c.CreditCardNumber) {
		logrus.Errorf("Invalid LHUN card ")
		return ErrInvalidCC
	}
	return nil
}

// AddToUrlValues : Add the card info to the given url.Values map for authorize.net
func (c CardInfoType) AddToUrlValues(urlValues url.Values) {
	st := reflect.TypeOf(c)
	sv := reflect.ValueOf(c)
	for i := 0; i < st.NumField(); i++ {
		field := st.Field(i)
		x_name := field.Tag.Get("authorizeNet")
		val := sv.FieldByIndex([]int{i}).String()
		if val != "" {
			// fmt.Printf("authorizeNet=%s value=%s\n", x_name, val)
			urlValues[x_name] = []string{val}
		}
	}
}

// --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------
// Lhun
// --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------

func CheckCCLhun(cc string) bool {
	var DELTA = []int{0, 1, 2, 3, 4, -4, -3, -2, -1, 0}
	checksum := 0
	bOdd := false
	card := []byte(cc)
	for i := len(card) - 1; i > -1; i-- {
		cn := int(card[i]) - 48
		checksum += cn

		if bOdd {
			checksum += DELTA[cn]
		}
		bOdd = !bOdd
	}
	if checksum%10 == 0 {
		return true
	}
	return false
}

const db1 = false
