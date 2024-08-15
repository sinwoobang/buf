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

package bufcheckserverbuild

import (
	"github.com/bufbuild/buf/private/buf/bufcheck/internal/bufcheckserver/internal/bufcheckserverhandle"
	"github.com/bufbuild/buf/private/buf/bufcheck/internal/bufcheckserver/internal/bufcheckserverutil"
	"github.com/bufbuild/bufplugin-go/check"
)

var (
	// BreakingEnumNoDeleteRuleSpecBuilder is a rule spec builder.
	BreakingEnumNoDeleteRuleSpecBuilder = &bufcheckserverutil.RuleSpecBuilder{
		ID:      "ENUM_NO_DELETE",
		Purpose: "Checks enums are not deleted from a given file.",
		Type:    check.RuleTypeBreaking,
		Handler: bufcheckserverhandle.HandleBreakingEnumNoDelete,
	}
	// BreakingExtensionNoDeleteRuleSpecBuilder is a rule spec builder.
	BreakingExtensionNoDeleteRuleSpecBuilder = &bufcheckserverutil.RuleSpecBuilder{
		ID:      "EXTENSION_NO_DELETE",
		Purpose: "Checks extensions are not deleted from a given file.",
		Type:    check.RuleTypeBreaking,
		Handler: bufcheckserverhandle.HandleBreakingExtensionNoDelete,
	}
	// BreakingFileNoDeleteRuleSpecBuilder is a rule spec builder.
	BreakingFileNoDeleteRuleSpecBuilder = &bufcheckserverutil.RuleSpecBuilder{
		ID:      "FILE_NO_DELETE",
		Purpose: "Checks files are not deleted.",
		Type:    check.RuleTypeBreaking,
		Handler: bufcheckserverhandle.HandleBreakingFileNoDelete,
	}
	// BreakingMessageNoDeleteRuleSpecBuilder is a rule spec builder.
	BreakingMessageNoDeleteRuleSpecBuilder = &bufcheckserverutil.RuleSpecBuilder{
		ID:      "MESSAGE_NO_DELETE",
		Purpose: "Checks messages are not deleted from a given file.",
		Type:    check.RuleTypeBreaking,
		Handler: bufcheckserverhandle.HandleBreakingMessageNoDelete,
	}
	// BreakingServiceNoDeleteRuleSpecBuilder is a rule spec builder.
	BreakingServiceNoDeleteRuleSpecBuilder = &bufcheckserverutil.RuleSpecBuilder{
		ID:      "SERVICE_NO_DELETE",
		Purpose: "Checks services are not deleted from a given file.",
		Type:    check.RuleTypeBreaking,
		Handler: bufcheckserverhandle.HandleBreakingServiceNoDelete,
	}
	// BreakingEnumSameTypeRuleSpecBuilder is a rule spec builder.
	BreakingEnumSameTypeRuleSpecBuilder = &bufcheckserverutil.RuleSpecBuilder{
		ID:      "ENUM_SAME_TYPE",
		Purpose: "Checks that enums have the same type (open vs closed).",
		Type:    check.RuleTypeBreaking,
		Handler: bufcheckserverhandle.HandleBreakingEnumSameType,
	}
	// BreakingEnumValueNoDeleteRuleSpecBuilder is a rule spec builder.
	BreakingEnumValueNoDeleteRuleSpecBuilder = &bufcheckserverutil.RuleSpecBuilder{
		ID:      "ENUM_VALUE_NO_DELETE",
		Purpose: "Check enum values are not deleted from a given enum.",
		Type:    check.RuleTypeBreaking,
		Handler: bufcheckserverhandle.HandleBreakingEnumValueNoDelete,
	}
	// BreakingExtensionMessageNoDeleteRuleSpecBuilder is a rule spec builder.
	BreakingExtensionMessageNoDeleteRuleSpecBuilder = &bufcheckserverutil.RuleSpecBuilder{
		ID:      "EXTENSION_MESSAGE_NO_DELETE",
		Purpose: "Checks extension ranges are not deleted from a given message.",
		Type:    check.RuleTypeBreaking,
		Handler: bufcheckserverhandle.HandleBreakingExtensionMessageNoDelete,
	}
	// BreakingFieldNoDeleteRuleSpecBuilder is a rule spec builder.
	BreakingFieldNoDeleteRuleSpecBuilder = &bufcheckserverutil.RuleSpecBuilder{
		ID:      "FIELD_NO_DELETE",
		Purpose: "Checks fields are not deleted from a given message.",
		Type:    check.RuleTypeBreaking,
		Handler: bufcheckserverhandle.HandleBreakingFieldNoDelete,
	}
	// BreakingFieldSameCardinalityRuleSpecBuilder is a rule spec builder.
	BreakingFieldSameCardinalityRuleSpecBuilder = &bufcheckserverutil.RuleSpecBuilder{
		ID:      "FIELD_SAME_CARDINALITY",
		Purpose: "Checks fields have the same cardinalities in a given message.",
		Type:    check.RuleTypeBreaking,
		Handler: bufcheckserverhandle.HandleBreakingFieldSameCardinality,
	}
	// BreakingFieldSameCppStringTypeRuleSpecBuilder is a rule spec builder.
	BreakingFieldSameCppStringTypeRuleSpecBuilder = &bufcheckserverutil.RuleSpecBuilder{
		ID:      "FIELD_SAME_CPP_STRING_TYPE",
		Purpose: "Checks fields have the same C++ string type, based on ctype field option or (pb.cpp).string_type feature.",
		Type:    check.RuleTypeBreaking,
		Handler: bufcheckserverhandle.HandleBreakingFieldSameCppStringType,
	}
	// BreakingFieldSameJavaUTF8ValidationRuleSpecBuilder is a rule spec builder.
	BreakingFieldSameJavaUTF8ValidationRuleSpecBuilder = &bufcheckserverutil.RuleSpecBuilder{
		ID:      "FIELD_SAME_JAVA_UTF8_VALIDATION",
		Purpose: "Checks fields have the same Java string UTF8 validation, based on java_string_check_utf8 file option or (pb.java).utf8_validation feature.",
		Type:    check.RuleTypeBreaking,
		Handler: bufcheckserverhandle.HandleBreakingFieldSameJavaUTF8Validation,
	}
	// BreakingFieldSameJSTypeRuleSpecBuilder is a rule spec builder.
	BreakingFieldSameJSTypeRuleSpecBuilder = &bufcheckserverutil.RuleSpecBuilder{
		ID:      "FIELD_SAME_JSTYPE",
		Purpose: "Checks fields have the same value for the jstype option.",
		Type:    check.RuleTypeBreaking,
		Handler: bufcheckserverhandle.HandleBreakingFieldSameJSType,
	}
	// BreakingFieldSameTypeRuleSpecBuilder is a rule spec builder.
	BreakingFieldSameTypeRuleSpecBuilder = &bufcheckserverutil.RuleSpecBuilder{
		ID:      "FIELD_SAME_TYPE",
		Purpose: "Checks fields have the same types in a given message.",
		Type:    check.RuleTypeBreaking,
		Handler: bufcheckserverhandle.HandleBreakingFieldSameType,
	}

	// BreakingFieldSameUTF8ValidationRuleSpecBuilder is a rule spec builder.
	BreakingFieldSameUTF8ValidationRuleSpecBuilder = &bufcheckserverutil.RuleSpecBuilder{
		ID:      "FIELD_SAME_UTF8_VALIDATION",
		Purpose: "Checks string fields have the same UTF8 validation mode.",
		Type:    check.RuleTypeBreaking,
		Handler: bufcheckserverhandle.HandleBreakingFieldSameUTF8Validation,
	}
	// BreakingFileSameCcEnableArenasRuleSpecBuilder is a rule spec builder.
	BreakingFileSameCcEnableArenasRuleSpecBuilder = &bufcheckserverutil.RuleSpecBuilder{
		ID:      "FILE_SAME_CC_ENABLE_ARENAS",
		Purpose: "Check files have the same value for the cc_enable_arenas option.",
		Type:    check.RuleTypeBreaking,
		Handler: bufcheckserverhandle.HandleBreakingFileSameCcEnableArenas,
	}

	// LintCommentEnumRuleSpecBuilder is a rule spec builder.
	LintCommentEnumRuleSpecBuilder = &bufcheckserverutil.RuleSpecBuilder{
		ID:      "COMMENT_ENUM",
		Purpose: "Checks that enums have non-empty comments.",
		Type:    check.RuleTypeLint,
		Handler: bufcheckserverhandle.HandleLintCommentEnum,
	}
	// LintCommentEnumValueRuleSpecBuilder is a rule spec builder.
	LintCommentEnumValueRuleSpecBuilder = &bufcheckserverutil.RuleSpecBuilder{
		ID:      "COMMENT_ENUM_VALUE",
		Purpose: "Checks that enum values have non-empty comments.",
		Type:    check.RuleTypeLint,
		Handler: bufcheckserverhandle.HandleLintCommentEnumValue,
	}
	// LintCommentFieldRuleSpecBuilder is a rule spec builder.
	LintCommentFieldRuleSpecBuilder = &bufcheckserverutil.RuleSpecBuilder{
		ID:      "COMMENT_FIELD",
		Purpose: "Checks that fields have non-empty comments.",
		Type:    check.RuleTypeLint,
		Handler: bufcheckserverhandle.HandleLintCommentField,
	}
	// LintCommentMessageRuleSpecBuilder is a rule spec builder.
	LintCommentMessageRuleSpecBuilder = &bufcheckserverutil.RuleSpecBuilder{
		ID:      "COMMENT_MESSAGE",
		Purpose: "Checks that messages have non-empty comments.",
		Type:    check.RuleTypeLint,
		Handler: bufcheckserverhandle.HandleLintCommentMessage,
	}
	// LintCommentOneofRuleSpecBuilder is a rule spec builder.
	LintCommentOneofRuleSpecBuilder = &bufcheckserverutil.RuleSpecBuilder{
		ID:      "COMMENT_ONEOF",
		Purpose: "Checks that oneofs have non-empty comments.",
		Type:    check.RuleTypeLint,
		Handler: bufcheckserverhandle.HandleLintCommentOneof,
	}
	// LintCommentRPCRuleSpecBuilder is a rule spec builder.
	LintCommentRPCRuleSpecBuilder = &bufcheckserverutil.RuleSpecBuilder{
		ID:      "COMMENT_RPC",
		Purpose: "Checks that RPCs have non-empty comments.",
		Type:    check.RuleTypeLint,
		Handler: bufcheckserverhandle.HandleLintCommentRPC,
	}
	// LintCommentServiceRuleSpecBuilder is a rule spec builder.
	LintCommentServiceRuleSpecBuilder = &bufcheckserverutil.RuleSpecBuilder{
		ID:      "COMMENT_SERVICE",
		Purpose: "Checks that services have non-empty comments.",
		Type:    check.RuleTypeLint,
		Handler: bufcheckserverhandle.HandleLintCommentService,
	}
	// LintDirectorySamePackageRuleSpecBuilder is a rule spec builder.
	LintDirectorySamePackageRuleSpecBuilder = &bufcheckserverutil.RuleSpecBuilder{
		ID:      "DIRECTORY_SAME_PACKAGE",
		Purpose: "Checks that all files in a given directory are in the same package.",
		Type:    check.RuleTypeLint,
		Handler: bufcheckserverhandle.HandleLintDirectorySamePackage,
	}
	// LintServicePascalCaseRuleSpecBuilder is a rule spec builder.
	LintServicePascalCaseRuleSpecBuilder = &bufcheckserverutil.RuleSpecBuilder{
		ID:      "SERVICE_PASCAL_CASE",
		Purpose: "Checks that services are PascalCase.",
		Type:    check.RuleTypeLint,
		Handler: bufcheckserverhandle.HandleLintServicePascalCase,
	}
	// LintServiceSuffixRuleSpecBuilder is a rule spec builder.
	LintServiceSuffixRuleSpecBuilder = &bufcheckserverutil.RuleSpecBuilder{
		ID:      "SERVICE_SUFFIX",
		Purpose: `Checks that services have a consistent suffix (configurable, default suffix is "Service").`,
		Type:    check.RuleTypeLint,
		Handler: bufcheckserverhandle.HandleLintServiceSuffix,
	}
)
