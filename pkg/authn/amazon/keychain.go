package amazon

import (
	"github.com/awslabs/amazon-ecr-credential-helper/ecr-login/api"
	"github.com/google/go-containerregistry/pkg/authn"
)

// Keychain exports an instance of the amazon Keychain.
var Keychain authn.Keychain = amazonKeychain{}

type amazonKeychain struct{}

// Resolve implements authn.Keychain a la docker-credential-ecr-login.
func (amazonKeychain) Resolve(target authn.Resource) (authn.Authenticator, error) {
	reg, err := api.ExtractRegistry(target.String())
	if err != nil {
		return authn.Anonymous, nil
	}

	cf := api.DefaultClientFactory{}
	var client api.Client
	if reg.FIPS {
		client, err = cf.NewClientWithFipsEndpoint(reg.Region)
		if err != nil {
			return authn.Anonymous, nil
		}
	} else {
		client = cf.NewClientFromRegion(reg.Region)
	}

	auth, err := client.GetCredentials(target.String())
	if err != nil {
		return authn.Anonymous, nil
	}
	return &authn.Basic{
		Username: auth.Username,
		Password: auth.Password,
	}, nil
}
