package internal

import (
	"encoding/base64"
	"encoding/json"
	"evsys-back/models"
	"evsys-back/services"
	"fmt"
	"net/url"
)

type Payments struct {
	database services.Database
	logger   services.LogHandler
}

func NewPayments() *Payments {
	return &Payments{}
}

func (p *Payments) SetDatabase(database services.Database) {
	p.database = database
}

func (p *Payments) SetLogger(logger services.LogHandler) {
	p.logger = logger
}

func (p *Payments) Notify(data []byte) error {

	params, err := url.ParseQuery(string(data))
	if err != nil {
		p.logger.Error("parse query", err)
		p.logger.Info(fmt.Sprintf("body: %s", string(data)))
		return err
	}

	paymentResult := models.PaymentResult{
		SignatureVersion: params.Get("Ds_SignatureVersion"),
		Parameters:       params.Get("Ds_MerchantParameters"),
		Signature:        params.Get("Ds_Signature"),
	}

	if paymentResult.Parameters != "" {
		go p.processNotifyData(&paymentResult)
	} else {
		p.logger.Warn(fmt.Sprintf("empty parameters: %s", string(data)))
		return fmt.Errorf("empty Ds_MerchantParameters")
	}
	return nil
}

func (p *Payments) processNotifyData(paymentResult *models.PaymentResult) {

	var jsonBytes, err = base64.StdEncoding.DecodeString(paymentResult.Parameters)
	if err != nil {
		p.logger.Error("decode base64", err)
		p.logger.Info(fmt.Sprintf("Ds_MerchantParameters: %s", paymentResult.Parameters[0:50]))
		return
	}

	var params models.PaymentParameters
	err = json.Unmarshal(jsonBytes, &params)
	if err != nil {
		p.logger.Error("unmarshal json", err)
		p.logger.Info(fmt.Sprintf("Ds_MerchantParameters: %s", string(jsonBytes)))
		return
	}

	err = p.database.SavePaymentResult(&params)
	if err != nil {
		p.logger.Error("save payment result", err)
	}
	p.logger.Info(fmt.Sprintf("order: %s; amount: %s", params.Order, params.Amount))
}
