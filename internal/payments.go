package internal

import (
	"encoding/base64"
	"encoding/json"
	"evsys-back/models"
	"evsys-back/services"
	"fmt"
	"net/url"
	"strconv"
	"sync"
	"time"
)

type Payments struct {
	database services.Database
	logger   services.LogHandler
	mutex    *sync.Mutex
}

func NewPayments() *Payments {
	return &Payments{
		mutex: &sync.Mutex{},
	}
}

func (p *Payments) Lock() {
	p.mutex.Lock()
}

func (p *Payments) Unlock() {
	p.mutex.Unlock()
}

func (p *Payments) SetDatabase(database services.Database) {
	p.database = database
}

func (p *Payments) SetLogger(logger services.LogHandler) {
	p.logger = logger
}

func (p *Payments) Notify(data []byte) error {
	p.Lock()
	defer p.Unlock()

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
	//p.logger.Info(fmt.Sprintf("Ds_MerchantParameters: %s", string(jsonBytes)))

	var params models.PaymentParameters
	err = json.Unmarshal(jsonBytes, &params)
	if err != nil {
		p.logger.Error("parameters: unmarshal json", err)
		p.logger.Info(fmt.Sprintf("Ds_MerchantParameters: %s", string(jsonBytes)))
		return
	}

	p.logger.Info(fmt.Sprintf("notify type: %s; result: %s; order: %s; amount: %s", params.TransactionType, params.Response, params.Order, params.Amount))
	err = p.database.SavePaymentResult(&params)
	if err != nil {
		p.logger.Error("save payment result", err)
	}

	number, err := strconv.Atoi(params.Order)
	if err != nil {
		p.logger.Error("read order number", err)
		return
	}
	amount, err := strconv.Atoi(params.Amount)
	if err != nil {
		p.logger.Error("read amount", err)
		return
	}
	order, err := p.database.GetPaymentOrder(number)
	if err != nil {
		p.logger.Error("get payment order", err)
		return
	}
	if !order.IsCompleted {
		order.Amount = amount
		order.IsCompleted = true
		order.Result = params.Response
		order.TimeClosed = time.Now()
		order.Currency = params.Currency
		order.Date = fmt.Sprintf("%s %s", params.Date, params.Hour)

		err = p.database.SavePaymentOrder(order)
		if err != nil {
			p.logger.Error("save payment order", err)
		}
	}

	err = p.checkPaymentResult(&params)
	if err != nil {
		p.logger.Warn(fmt.Sprintf("error %s; transaction %v; order %s; amount %s", params.Response, order.TransactionId, params.Order, params.Amount))
		return
	}

	// if transaction type is 3, then it is a refund
	if params.TransactionType == "3" {
		order.RefundAmount = amount
		order.RefundTime = time.Now()
		err = p.database.SavePaymentOrder(order)
		if err != nil {
			p.logger.Error("save payment order", err)
		}
		return
	}

	if order.TransactionId > 0 {

		transaction, err := p.database.GetTransaction(order.TransactionId)
		if err != nil {
			p.logger.Error("get transaction", err)
			return
		}
		transaction.PaymentOrder = order.Order
		transaction.PaymentBilled = order.Amount

		err = p.database.UpdateTransaction(transaction)
		if err != nil {
			p.logger.Error("update transaction", err)
			return
		}

	} else {

		paymentMethod := models.PaymentMethod{
			Description: "**** **** **** ****",
			Identifier:  params.MerchantIdentifier,
			CardBrand:   params.CardBrand,
			CardCountry: params.CardCountry,
			ExpiryDate:  params.ExpiryDate,
			UserId:      order.UserId,
			UserName:    order.UserName,
		}
		err = p.database.SavePaymentMethod(&paymentMethod)
		if err != nil {
			p.logger.Error("save payment method", err)
			return
		}

	}

}

func (p *Payments) checkPaymentResult(result *models.PaymentParameters) error {
	if result.TransactionType == "0" {
		if result.Response != "0000" {
			return fmt.Errorf("code %s", result.Response)
		}
		return nil
	}
	if result.TransactionType == "3" {
		if result.Response != "0900" {
			return fmt.Errorf("code %s", result.Response)
		}
		return nil
	}
	return fmt.Errorf("code %s; transaction type %s", result.Response, result.TransactionType)
}

func (p *Payments) SavePaymentMethod(user *models.User, data []byte) error {
	p.Lock()
	defer p.Unlock()

	paymentMethod, err := p.unmarshallPaymentMethod(data)
	if err != nil {
		p.logger.Error("method: unmarshal json", err)
		return err
	}

	paymentMethod.UserId = user.UserId
	paymentMethod.UserName = user.Username

	err = p.database.SavePaymentMethod(paymentMethod)
	if err != nil {
		p.logger.Error("save payment method", err)
		return err
	}
	return nil
}

func (p *Payments) UpdatePaymentMethod(user *models.User, data []byte) error {
	p.Lock()
	defer p.Unlock()

	paymentMethod, err := p.unmarshallPaymentMethod(data)
	if err != nil {
		p.logger.Error("method: unmarshal json", err)
		return err
	}
	paymentMethod.UserId = user.UserId

	err = p.database.UpdatePaymentMethod(paymentMethod)
	if err != nil {
		p.logger.Error("update payment method", err)
		return err
	}
	return nil
}

func (p *Payments) DeletePaymentMethod(user *models.User, data []byte) error {
	p.Lock()
	defer p.Unlock()

	paymentMethod, err := p.unmarshallPaymentMethod(data)
	if err != nil {
		p.logger.Error("delete method: unmarshal json", err)
		return err
	}
	if paymentMethod.Identifier == "" {
		p.logger.Warn("method: empty identifier")
		return fmt.Errorf("empty identifier")
	}
	paymentMethod.UserId = user.UserId

	err = p.database.DeletePaymentMethod(paymentMethod)
	if err != nil {
		p.logger.Error("delete payment method", err)
		return err
	}
	return nil
}

func (p *Payments) SetOrder(user *models.User, data []byte) (*models.PaymentOrder, error) {
	p.Lock()
	defer p.Unlock()

	var order models.PaymentOrder
	err := json.Unmarshal(data, &order)
	if err != nil {
		p.logger.Error("order: unmarshal json", err)
		p.logger.Info(fmt.Sprintf("method data: %s", string(data)))
		return nil, err
	}

	if order.Order == 0 {

		// if payment process was interrupted, find unclosed order and close it
		if order.TransactionId > 0 {
			continueOrder, _ := p.database.GetPaymentOrderByTransaction(order.TransactionId)
			if continueOrder != nil {
				continueOrder.IsCompleted = true
				continueOrder.Result = "closed without response"
				continueOrder.TimeClosed = time.Now()
				_ = p.database.SavePaymentOrder(continueOrder)
			}
		}

		lastOrder, _ := p.database.GetLastOrder()
		if lastOrder != nil {
			order.Order = lastOrder.Order + 1
		} else {
			order.Order = 1200
		}
		order.TimeOpened = time.Now()
	}

	order.UserId = user.UserId
	order.UserName = user.Username

	err = p.database.SavePaymentOrder(&order)
	if err != nil {
		p.logger.Error("save order", err)
		return nil, err
	}
	return &order, nil
}

// unmarshall payment method from bytes
func (p *Payments) unmarshallPaymentMethod(data []byte) (*models.PaymentMethod, error) {
	var paymentMethod models.PaymentMethod
	err := json.Unmarshal(data, &paymentMethod)
	if err != nil {
		p.logger.Info(fmt.Sprintf("method data: %s", string(data)))
		return nil, err
	}
	if paymentMethod.Identifier == "" {
		return nil, fmt.Errorf("empty identifier")
	}
	return &paymentMethod, nil
}
