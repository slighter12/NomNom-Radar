package handler

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"radar/internal/domain/entity"
	"radar/internal/usecase"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recordingProfileUsecase struct {
	updateDiscoveryProfileInput *usecase.UpdateMerchantDiscoveryProfileInput
	updateMerchantProfileInput  *usecase.UpdateMerchantProfileInput
}

func (uc *recordingProfileUsecase) GetProfile(_ context.Context, _ uuid.UUID) (*entity.User, error) {
	return nil, nil
}

func (uc *recordingProfileUsecase) UpdateUserProfile(_ context.Context, _ uuid.UUID, _ *usecase.UpdateUserProfileInput) error {
	return nil
}

func (uc *recordingProfileUsecase) UpdateMerchantProfile(_ context.Context, _ uuid.UUID, input *usecase.UpdateMerchantProfileInput) error {
	uc.updateMerchantProfileInput = input

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
	uc.updateDiscoveryProfileInput = input

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
	require.NotNil(t, profileUC.updateDiscoveryProfileInput)
	assert.True(t, profileUC.updateDiscoveryProfileInput.ActiveHubID.IsSet)
	assert.Nil(t, profileUC.updateDiscoveryProfileInput.ActiveHubID.Value)
	require.NotNil(t, profileUC.updateDiscoveryProfileInput.IsPublic)
	assert.False(t, *profileUC.updateDiscoveryProfileInput.IsPublic)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestUserHandler_UpdateMerchantProfile_MapsFields(t *testing.T) {
	storeName := "NomNom Bento"
	storeNameInput := "  NomNom Bento  "
	storeDescription := "烤肉店，主打台式夜市料理"

	profileUC := &recordingProfileUsecase{}
	handler := &UserHandler{profileUC: profileUC}
	c, rec := newJSONContext(http.MethodPatch, "/merchant/profile", `{"store_name":"`+storeNameInput+`","store_description":"`+storeDescription+`"}`)
	c.Set("userID", uuid.New())

	err := handler.UpdateMerchantProfile(c)

	require.NoError(t, err)
	require.NotNil(t, profileUC.updateMerchantProfileInput)
	require.NotNil(t, profileUC.updateMerchantProfileInput.StoreName)
	assert.Equal(t, storeName, *profileUC.updateMerchantProfileInput.StoreName)
	require.NotNil(t, profileUC.updateMerchantProfileInput.StoreDescription)
	assert.Equal(t, storeDescription, *profileUC.updateMerchantProfileInput.StoreDescription)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestUserHandler_UpdateMerchantProfile_AtLeastOneFieldRequired(t *testing.T) {
	profileUC := &recordingProfileUsecase{}
	handler := &UserHandler{profileUC: profileUC}
	c, rec := newJSONContext(http.MethodPatch, "/merchant/profile", `{}`)
	c.Set("userID", uuid.New())

	err := handler.UpdateMerchantProfile(c)
	writeTestErrorResponse(c, err)

	require.Error(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), `"code":"VALIDATION_FAILED"`)
	assert.Contains(t, rec.Body.String(), `"message":"輸入資料驗證失敗"`)
	assert.Contains(t, rec.Body.String(), `"details":"at least one of store_name or store_description is required"`)
}

func TestUserHandler_UpdateMerchantProfile_StoreNameCannotBeBlank(t *testing.T) {
	profileUC := &recordingProfileUsecase{}
	handler := &UserHandler{profileUC: profileUC}
	c, rec := newJSONContext(http.MethodPatch, "/merchant/profile", `{"store_name":"   ","store_description":"test"}`)
	c.Set("userID", uuid.New())

	err := handler.UpdateMerchantProfile(c)
	writeTestErrorResponse(c, err)

	require.Error(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), `"code":"VALIDATION_FAILED"`)
	assert.Contains(t, rec.Body.String(), `"message":"輸入資料驗證失敗"`)
	assert.Contains(t, rec.Body.String(), `"details":"store_name is required"`)
}

func TestUserHandler_UpdateMerchantProfile_StoreNameTooLong(t *testing.T) {
	profileUC := &recordingProfileUsecase{}
	handler := &UserHandler{profileUC: profileUC}
	c, rec := newJSONContext(http.MethodPatch, "/merchant/profile", `{"store_name":"`+strings.Repeat("a", 51)+`"}`)
	c.Set("userID", uuid.New())

	err := handler.UpdateMerchantProfile(c)
	writeTestErrorResponse(c, err)

	require.Error(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), `"code":"VALIDATION_FAILED"`)
	assert.Contains(t, rec.Body.String(), `"message":"輸入資料驗證失敗"`)
	assert.Contains(t, rec.Body.String(), `"details":"store_name must be less than or equal to 50"`)
}

func TestUserHandler_UpdateMerchantProfile_StoreDescriptionTooLong(t *testing.T) {
	profileUC := &recordingProfileUsecase{}
	handler := &UserHandler{profileUC: profileUC}
	c, rec := newJSONContext(http.MethodPatch, "/merchant/profile", `{"store_description":"`+strings.Repeat("a", 501)+`"}`)
	c.Set("userID", uuid.New())

	err := handler.UpdateMerchantProfile(c)
	writeTestErrorResponse(c, err)

	require.Error(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), `"code":"VALIDATION_FAILED"`)
	assert.Contains(t, rec.Body.String(), `"message":"輸入資料驗證失敗"`)
	assert.Contains(t, rec.Body.String(), `"details":"store_description must be less than or equal to 500"`)
}

func TestUserHandler_UpdateMerchantProfile_OnlyStoreDescription(t *testing.T) {
	profileUC := &recordingProfileUsecase{}
	handler := &UserHandler{profileUC: profileUC}
	c, rec := newJSONContext(http.MethodPatch, "/merchant/profile", `{"store_description":"updated description"}`)
	c.Set("userID", uuid.New())

	err := handler.UpdateMerchantProfile(c)
	writeTestErrorResponse(c, err)

	require.NoError(t, err)
	require.NotNil(t, profileUC.updateMerchantProfileInput)
	assert.Nil(t, profileUC.updateMerchantProfileInput.StoreName)
	require.NotNil(t, profileUC.updateMerchantProfileInput.StoreDescription)
	assert.Equal(t, "updated description", *profileUC.updateMerchantProfileInput.StoreDescription)
	assert.Equal(t, http.StatusOK, rec.Code)
}
