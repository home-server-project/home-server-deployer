package backup

import (
	"github.com/home-server-project/home-server-deployer/internal/model"
	"testing"
)

func TestContractRejectsUnnamedProvider(t *testing.T) {
	if err := ValidateContract(model.BackupContract{Strategy: "provider", Consistency: "provider-managed"}); err == nil {
		t.Fatal("expected provider validation error")
	}
}
