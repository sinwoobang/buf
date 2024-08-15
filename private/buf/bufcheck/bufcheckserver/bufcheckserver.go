// Copyright 2020-2024 Buf Technologies, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package bufcheckserver

import (
	"github.com/bufbuild/buf/private/buf/bufcheck/bufcheckserver/internal/bufcheckserverbuild"
	"github.com/bufbuild/buf/private/buf/bufcheck/bufcheckserver/internal/bufcheckserverutil"
	"github.com/bufbuild/bufplugin-go/check"
)

var (
	// V1Beta1Spec is the v1beta1 check.Spec.
	V1Beta1Spec = &check.Spec{
		Rules:  []*check.RuleSpec{},
		Before: bufcheckserverutil.Before,
	}

	// V1Spec is the v1beta1 check.Spec.
	V1Spec = &check.Spec{
		Rules:  []*check.RuleSpec{},
		Before: bufcheckserverutil.Before,
	}

	// V2Spec is the v2 check.Spec.
	V2Spec = &check.Spec{
		Rules: []*check.RuleSpec{
			bufcheckserverbuild.BreakingEnumSameTypeRuleSpecBuilder.Build(false, []string{"FILE", "PACKAGE"}),
			bufcheckserverbuild.LintCommentEnumRuleSpecBuilder.Build(false, []string{"COMMENTS"}),
			bufcheckserverbuild.LintCommentEnumValueRuleSpecBuilder.Build(false, []string{"COMMENTS"}),
			bufcheckserverbuild.LintCommentFieldRuleSpecBuilder.Build(false, []string{"COMMENTS"}),
			bufcheckserverbuild.LintCommentMessageRuleSpecBuilder.Build(false, []string{"COMMENTS"}),
			bufcheckserverbuild.LintCommentOneofRuleSpecBuilder.Build(false, []string{"COMMENTS"}),
			bufcheckserverbuild.LintCommentRPCRuleSpecBuilder.Build(false, []string{"COMMENTS"}),
			bufcheckserverbuild.LintCommentServiceRuleSpecBuilder.Build(false, []string{"COMMENTS"}),
			bufcheckserverbuild.LintDirectorySamePackageRuleSpecBuilder.Build(true, []string{"MINIMAL", "BASIC", "DEFAULT"}),
			bufcheckserverbuild.LintServicePascalCaseRuleSpecBuilder.Build(true, []string{"BASIC", "DEFAULT"}),
			bufcheckserverbuild.LintServiceSuffixRuleSpecBuilder.Build(true, []string{"DEFAULT"}),
		},
		Before: bufcheckserverutil.Before,
	}
)
