package learned

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestRuntimeRequiresExplicitReplacementAndSwitchesPinnedModels(t *testing.T) {
	learnedRuntime.Lock()
	learnedRuntime.session = nil
	learnedRuntime.config = SessionConfig{}
	learnedRuntime.Unlock()
	t.Cleanup(func() {
		learnedRuntime.Lock()
		learnedRuntime.session = nil
		learnedRuntime.config = SessionConfig{}
		learnedRuntime.Unlock()
	})

	serverRoot := testServerRoot(t)
	base := runtimeRequest{
		Op:              "initialize",
		ContentRoot:     filepath.Join(serverRoot, "content"),
		RunStateRoot:    filepath.Join(serverRoot, "save", "run_players"),
		DiagnosticsPath: filepath.Join(t.TempDir(), "learned-switch.jsonl"),
		TimeoutMS:       2_000,
	}
	base.ModelPath = testModelPath(t)
	old := runtimeCall(t, base)
	if old["ok"] != true {
		t.Fatalf("old v1 runtime initialization failed: %#v", old)
	}

	base.ModelPath = testV2ModelPath(t)
	base.ModelSHA256 = "0e6ea5d84c316a7709c9c1b98b983e8f76e486c6e1625c479c55d2860bd23a86"
	refused := runtimeCall(t, base)
	if refused["ok"] == true {
		t.Fatal("runtime silently replaced an initialized model")
	}

	base.ReplaceSession = true
	replaced := runtimeCall(t, base)
	if replaced["ok"] != true {
		t.Fatalf("explicit v2 runtime replacement failed: %#v", replaced)
	}
	result := replaced["result"].(map[string]any)
	model := result["model"].(map[string]any)
	if model["model_id"] != "blade-warden-decision-quality-seed-22-v2" {
		t.Fatalf("runtime replacement loaded the wrong model: %#v", model)
	}

	base.ModelPath = testV3ModelPath(t)
	base.ModelSHA256 = "529a6b4d6ad347d5ba86b5e000cb5fceec306414cdf0af3405713a2bc5c32ebb"
	v3 := runtimeCall(t, base)
	if v3["ok"] != true {
		t.Fatalf("explicit v3 runtime replacement failed: %#v", v3)
	}
	v3Model := v3["result"].(map[string]any)["model"].(map[string]any)
	if v3Model["model_id"] != "blade-warden-optimized-5m-seed-22-v3" {
		t.Fatalf("runtime replacement loaded the wrong v3 model: %#v", v3Model)
	}

	base.ModelPath = testGlobalChampionModelPath(t)
	base.ModelSHA256 = "96755c199f4d93261695d4928a0f95e00858d3a9d6a5c429e46911fbd0a3cac6"
	globalChampion := runtimeCall(t, base)
	if globalChampion["ok"] != true {
		t.Fatalf("explicit global-champion runtime replacement failed: %#v", globalChampion)
	}
	globalChampionModel := globalChampion["result"].(map[string]any)["model"].(map[string]any)
	if globalChampionModel["model_id"] != "blade-warden-global-champion-cp480-v3" {
		t.Fatalf("runtime replacement loaded the wrong global-champion model: %#v", globalChampionModel)
	}

	base.ModelPath = testModelPath(t)
	base.ModelSHA256 = ""
	restored := runtimeCall(t, base)
	if restored["ok"] != true {
		t.Fatalf("explicit switch back to v1 failed: %#v", restored)
	}
	restoredModel := restored["result"].(map[string]any)["model"].(map[string]any)
	if restoredModel["model_id"] != AcceptedModelID {
		t.Fatalf("runtime switch back loaded the wrong model: %#v", restoredModel)
	}
}

func runtimeCall(t *testing.T, request runtimeRequest) map[string]any {
	t.Helper()
	payload, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(HandleRuntimeRequest(string(payload))), &result); err != nil {
		t.Fatal(err)
	}
	return result
}
