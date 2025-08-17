package main

import (
	"radar/internal/infra/persistence/model"

	"gorm.io/gen"
)

func main() {
	models := []any{
		model.UserModel{},
		model.UserProfileModel{},
		model.MerchantProfileModel{},
		model.AuthenticationModel{},
		model.RefreshTokenModel{},
		model.AddressModel{},
	}

	gen := gen.NewGenerator(gen.Config{
		OutPath: "./internal/infra/persistence/postgres/query",
	})

	gen.ApplyBasic(models...)

	gen.Execute()
}
