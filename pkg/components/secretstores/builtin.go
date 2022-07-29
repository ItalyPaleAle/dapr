package secretstores

import (
	"os"

	"github.com/dapr/dapr/utils"
)

var builtinKubernetesSecretStorePrivate bool

func init() {
	builtinKubernetesSecretStorePrivate = utils.IsTruthy(os.Getenv("BUILTIN_KUBERNETES_SECRET_STORE_PRIVATE"))
}

// IsSecretStorePrivate returns true if the secret store with the given name is private and cannot be used by applications (only by the runtime itself).
func IsSecretStorePrivate(name string) bool {
	return builtinKubernetesSecretStorePrivate && name == BuiltinKubernetesSecretStore
}
