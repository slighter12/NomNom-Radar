package handler

import (
	"context"
	"net/http"
	"testing"

	"radar/internal/domain/entity"
	"radar/internal/usecase"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recordingProfileUsecase struct {
	updateInput *usecase.UpdateMerchantDiscoveryProfileInput
}

func (uc *recordingProfileUsecase) GetProfile(_ context.Context, _ uuid.UUID) (*entity.User, error) {
	return nil, nil
}

func (uc *recordingProfileUsecase) UpdateUserProfile(_ context.Context, _ uuid.UUID, _ *usecase.UpdateUserProfileInput) error {
	return nil
}

func (uc *recordingProfileUsecase) UpdateMerchantProfile(_ context.Context, _ uuid.UUID, _ *usecase.UpdateMerchantProfileInput) error {
	return nil
}

func (uc *recordingProfileUsecase) GetMerchantDiscoveryProfile(_ context.Context, _ uuid.UUID) (*usecase.MerchantDiscoveryProfileResult, error) {
	return &usecase.MerchantDiscoveryProfileResult{}, nil
}

func (uc *recordingProfileUsecase) UpdateMerchantDiscoveryProfile(
	_ context.Context,
	_ uuid.UUID,
	input *usecase.UpdateMerchantDiscoveryProfileInput,
) (*usecase.MerchantDiscoveryProfileResult, error) {
	uc.updateInput = input

	return &usecase.MerchantDiscoveryProfileResult{}, nil
}

func (uc *recordingProfileUsecase) SubmitMerchantVerification(_ context.Context, _ uuid.UUID, _ *usecase.SubmitMerchantVerificationInput) error {
	return nil
}

func (uc *recordingProfileUsecase) SwitchToMerchant(_ context.Context, _ uuid.UUID, _ *usecase.SwitchToMerchantInput) error {
	return nil
}

func (uc *recordingProfileUsecase) GetUserRole(_ context.Context, _ uuid.UUID) ([]string, error) {
	return nil, nil
}

func TestUserHandler_UpdateMerchantDiscoveryProfile_ParsesNullActiveHubAsClear(t *testing.T) {
	profileUC := &recordingProfileUsecase{}
	handler := &UserHandler{profileUC: profileUC}
	c, rec := newJSONContext(http.MethodPatch, "/merchant/discovery-profile", `{"active_hub_id":null,"is_public":false}`)
	c.Set("userID", uuid.New())

	err := handler.UpdateMerchantDiscoveryProfile(c)

	require.NoError(t, err)
	require.NotNil(t, profileUC.updateInput)
	assert.True(t, profileUC.updateInput.ActiveHubID.IsSet)
	assert.Nil(t, profileUC.updateInput.ActiveHubID.Value)
	require.NotNil(t, profileUC.updateInput.IsPublic)
	assert.False(t, *profileUC.updateInput.IsPublic)
	assert.Equal(t, http.StatusOK, rec.Code)
}
