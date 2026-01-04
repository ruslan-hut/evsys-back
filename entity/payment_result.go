package entity

type PaymentResult struct {
	SignatureVersion string `json:"Ds_SignatureVersion" validate:"required"`
	Parameters       string `json:"Ds_MerchantParameters" validate:"required"`
	Signature        string `json:"Ds_Signature" validate:"required"`
}
