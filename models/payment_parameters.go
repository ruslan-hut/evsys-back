package models

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
