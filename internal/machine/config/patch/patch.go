// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package patch

import (
	"bytes"
	"encoding/json"
	"fmt"

	jsonpatch "github.com/evanphx/json-patch/v5"
	"github.com/goccy/go-yaml"

	v1 "altlinux.space/alt-atomic/selenite/internal/machine/config/v1"
)

func Patch(cfg *v1.MachineConfig, patches [][]byte) (*v1.MachineConfig, error) {
	dst, err := json.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("patch: json.Marshal: %w", err)
	}

	for _, patch := range patches {
		jsonPatch, err := yaml.YAMLToJSON(bytes.TrimSpace(patch))
		if err != nil {
			return nil, fmt.Errorf("patch: yaml.YAMLToJSON: %w", err)
		}

		switch jsonPatch[0] {
		case '[':
			// RFC 6902 (JSON Patch)
			decodedPatch, err := jsonpatch.DecodePatch(jsonPatch)
			if err != nil {
				return nil, fmt.Errorf("patch: jsonpatch.DecodePatch: %w", err)
			}

			dst, err = decodedPatch.Apply(dst)
			if err != nil {
				return nil, fmt.Errorf("patch: decodedPatch.Apply: %w", err)
			}

		case '{':
			// RFC 7396 (JSON Merge Patch)
			dst, err = jsonpatch.MergePatch(dst, jsonPatch)
			if err != nil {
				return nil, fmt.Errorf("patch: jsonpatch.MergePatch: %w", err)
			}

		default:
			return nil, fmt.Errorf("patch: incorrect patch")
		}
	}

	var newCfg v1.MachineConfig

	if err := json.Unmarshal(dst, &newCfg); err != nil {
		return nil, fmt.Errorf("patch: json.Unmarshal: %w", err)
	}

	return &newCfg, nil
}
