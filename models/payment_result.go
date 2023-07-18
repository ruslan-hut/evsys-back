package models

type PaymentResult struct {
	SignatureVersion string `json:"Ds_SignatureVersion"`
	Parameters       string `json:"Ds_MerchantParameters"`
	Signature        string `json:"Ds_Signature"`
}
