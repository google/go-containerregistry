package amazon

import (
	"regexp"

	"github.com/awslabs/amazon-ecr-credential-helper/ecr-login/api"
	"github.com/google/go-containerregistry/pkg/authn"
)

// Keychain exports an instance of the amazon Keychain.
var Keychain authn.Keychain = amazonKeychain{}

type amazonKeychain struct{}

var ecrRE = regexp.MustCompile("[0-9]+.dkr.ecr.[a-z0-9-]+.amazonaws.com")

// Resolve implements authn.Keychain a la docker-credential-ecr-login.
//
// This behaves similarly to the ECR credential helper, but reuses tokens until
// they expire.
//
// We can't easily add this behavior to our credential helper implementation
// of authn.Authenticator because the credential helper protocol doesn't include
// expiration information, see here:
// https://godoc.org/github.com/docker/docker-credential-helpers/credentials#Credentials
func (amazonKeychain) Resolve(target authn.Resource) (authn.Authenticator, error) {
	// Only authenticate GCR and AR so it works with authn.NewMultiKeychain to fallback.
	host := target.RegistryStr()
	if host != "public.ecr.aws" && !ecrRE.MatchString(host) {
		return authn.Anonymous, nil
	}

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
