package entity

import (
	"strconv"
	"strings"
)

// Redsys Ds_TransactionType values.
const (
	RedsysTxPay          = "0" // authorization / direct payment
	RedsysTxPreauthorize = "1" // preauthorization
	RedsysTxCapture      = "2" // capture / confirmation of a preauthorization
	RedsysTxRefund       = "3" // refund
	RedsysTxCancel       = "9" // cancellation of a preauthorization
)

type PaymentParameters struct {
	MerchantCode       string `json:"Ds_MerchantCode" bson:"merchant_code"`
	Terminal           string `json:"Ds_Terminal" bson:"terminal"`
	Order              string `json:"Ds_Order" bson:"order"`
	Amount             string `json:"Ds_Amount" bson:"amount"`
	Currency           string `json:"Ds_Currency" bson:"currency"`
	Date               string `json:"Ds_Date" bson:"date"`
	Hour               string `json:"Ds_Hour" bson:"hour"`
	SecurePayment      string `json:"Ds_SecurePayment" bson:"secure_payment"`
	ExpiryDate         string `json:"Ds_ExpiryDate" bson:"expiry_date"`
	MerchantIdentifier string `json:"Ds_Merchant_Identifier" bson:"merchant_identifier"`
	CardCountry        string `json:"Ds_Card_Country" bson:"card_country"`
	Response           string `json:"Ds_Response" bson:"response"`
	MerchantData       string `json:"Ds_MerchantData" bson:"merchant_data"`
	TransactionType    string `json:"Ds_TransactionType" bson:"transaction_type"`
	ConsumerLanguage   string `json:"Ds_ConsumerLanguage" bson:"consumer_language"`
	AuthorisationCode  string `json:"Ds_AuthorisationCode" bson:"authorisation_code"`
	CardBrand          string `json:"Ds_Card_Brand" bson:"card_brand"`
	MerchantCofTxnid   string `json:"Ds_Merchant_Cof_Txnid" bson:"merchant_cof_txnid"`
	ProcessedPayMethod string `json:"Ds_ProcessedPayMethod" bson:"processed_pay_method"`
}

// IsApproved reports whether this Redsys notification represents an
// approved operation. See IsRedsysApproved.
func (p *PaymentParameters) IsApproved() bool {
	return IsRedsysApproved(p.TransactionType, p.Response)
}

// IsRedsysApproved reports whether a Redsys Ds_Response code indicates an
// approved operation. The single approval code depends on the operation
// type — every other code, including an empty or malformed one, is an
// error:
//
//   - payments / preauthorizations (type 0, 1): 0000–0099
//   - captures / refunds (type 2, 3):           0900
//   - cancellations (type 9):                   0400
//
// An unknown or empty transaction type is treated as a payment, since
// that is the only operation initiated from a context where the type may
// be missing (the hosted-form web flow).
func IsRedsysApproved(transactionType, responseCode string) bool {
	code, err := strconv.Atoi(strings.TrimSpace(responseCode))
	if err != nil {
		return false
	}
	switch transactionType {
	case RedsysTxCapture, RedsysTxRefund:
		return code == 900
	case RedsysTxCancel:
		return code == 400
	default:
		return code >= 0 && code <= 99
	}
}
