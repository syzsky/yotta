package workflow

import (
	"context"
	"github.com/yottaapp/yotta/internal/registryclient"
)

type checkoutClient interface {
	NativePayment(context.Context, string, bool) (registryclient.PaymentView, error)
	RenewPaymentSession(context.Context, string) (registryclient.PaymentSession, error)
	PaymentMethods(context.Context) ([]registryclient.PaymentMethod, error)
	Checkout(context.Context, string, registryclient.CheckoutInput) (registryclient.CheckoutPayment, error)
	CheckoutState(context.Context, string, string) (registryclient.CheckoutStatus, error)
}

func (s *Service) RegistryNativePayment(ctx context.Context, orderNo string, confirm bool) (registryclient.PaymentView, error) {
	c, ok := s.registry.(checkoutClient)
	if !ok {
		return registryclient.PaymentView{}, unavailable("registry")
	}
	result, err := c.NativePayment(ctx, orderNo, confirm)
	if err != nil {
		return registryclient.PaymentView{}, registryError("native_payment", err)
	}
	return result, nil
}

func (s *Service) RenewRegistryPaymentSession(ctx context.Context, orderNo string) (registryclient.PaymentSession, error) {
	c, ok := s.registry.(checkoutClient)
	if !ok {
		return registryclient.PaymentSession{}, unavailable("registry")
	}
	result, err := c.RenewPaymentSession(ctx, orderNo)
	if err != nil {
		return registryclient.PaymentSession{}, registryError("payment_session", err)
	}
	return result, nil
}

func (s *Service) RegistryPaymentMethods(ctx context.Context) ([]registryclient.PaymentMethod, error) {
	c, ok := s.registry.(checkoutClient)
	if !ok {
		return nil, unavailable("registry")
	}
	result, err := c.PaymentMethods(ctx)
	if err != nil {
		return nil, registryError("payment_methods", err)
	}
	return result, nil
}

func (s *Service) CreateRegistryCheckout(ctx context.Context, workflowID string, input registryclient.CheckoutInput) (registryclient.CheckoutPayment, error) {
	c, ok := s.registry.(checkoutClient)
	if !ok {
		return registryclient.CheckoutPayment{}, unavailable("registry")
	}
	result, err := c.Checkout(ctx, workflowID, input)
	if err != nil {
		return registryclient.CheckoutPayment{}, registryError("checkout", err)
	}
	return result, nil
}

func (s *Service) RegistryCheckoutState(ctx context.Context, orderNo, action string) (registryclient.CheckoutStatus, error) {
	c, ok := s.registry.(checkoutClient)
	if !ok {
		return registryclient.CheckoutStatus{}, unavailable("registry")
	}
	result, err := c.CheckoutState(ctx, orderNo, action)
	if err != nil {
		return registryclient.CheckoutStatus{}, registryError("checkout_state", err)
	}
	return result, nil
}
