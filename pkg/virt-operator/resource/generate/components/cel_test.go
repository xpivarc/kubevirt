package components_test

// This is modified from https://github.com/kubernetes/kubernetes/blob/25e11cd1c143ef136418c33bfbbbd4f24e32e529/staging/src/k8s.io/apiserver/pkg/admission/plugin/policy/validating/plugin.go#L129

import (
	"sync"

	v1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apiserver/pkg/admission/plugin/cel"
	"k8s.io/apiserver/pkg/admission/plugin/policy/validating"
	"k8s.io/apiserver/pkg/admission/plugin/webhook/matchconditions"
	"k8s.io/apiserver/pkg/cel/environment"
)

type Policy = v1.ValidatingAdmissionPolicy

func CompilePolicy(policy *Policy) validating.Validator {
	strictCost := false
	hasParam := false
	if policy.Spec.ParamKind != nil {
		hasParam = true
	}
	optionalVars := cel.OptionalVariableDeclarations{HasParams: hasParam, HasAuthorizer: false, StrictCost: strictCost}
	expressionOptionalVars := cel.OptionalVariableDeclarations{HasParams: hasParam, HasAuthorizer: false, StrictCost: strictCost}
	failurePolicy := policy.Spec.FailurePolicy
	var matcher matchconditions.Matcher = nil
	matchConditions := policy.Spec.MatchConditions
	var compositionEnvTemplate *cel.CompositionEnv
	compositionEnvTemplate = getCompositionEnvTemplateWithoutStrictCost()
	filterCompiler := cel.NewCompositedCompilerFromTemplate(compositionEnvTemplate)
	filterCompiler.CompileAndStoreVariables(convertv1beta1Variables(policy.Spec.Variables), optionalVars, environment.StoredExpressions)

	if len(matchConditions) > 0 {
		matchExpressionAccessors := make([]cel.ExpressionAccessor, len(matchConditions))
		for i := range matchConditions {
			matchExpressionAccessors[i] = (*matchconditions.MatchCondition)(&matchConditions[i])
		}
		matcher = matchconditions.NewMatcher(filterCompiler.CompileCondition(matchExpressionAccessors, optionalVars, environment.StoredExpressions), failurePolicy, "policy", "validate", policy.Name)
	}
	res := validating.NewValidator(
		filterCompiler.CompileCondition(convertv1Validations(policy.Spec.Validations), optionalVars, environment.StoredExpressions),
		matcher,
		filterCompiler.CompileCondition(convertv1AuditAnnotations(policy.Spec.AuditAnnotations), optionalVars, environment.StoredExpressions),
		filterCompiler.CompileCondition(convertv1MessageExpressions(policy.Spec.Validations), expressionOptionalVars, environment.StoredExpressions),
		failurePolicy,
	)

	return res
}

var lazyCompositionEnvTemplateWithoutStrictCostInit sync.Once
var lazyCompositionEnvTemplateWithoutStrictCost *cel.CompositionEnv

func getCompositionEnvTemplateWithoutStrictCost() *cel.CompositionEnv {
	lazyCompositionEnvTemplateWithoutStrictCostInit.Do(func() {
		env, err := cel.NewCompositionEnv(cel.VariablesTypeName, environment.MustBaseEnvSet(environment.DefaultCompatibilityVersion(), false))
		if err != nil {
			panic(err)
		}
		lazyCompositionEnvTemplateWithoutStrictCost = env
	})
	return lazyCompositionEnvTemplateWithoutStrictCost
}

func convertv1Validations(inputValidations []v1.Validation) []cel.ExpressionAccessor {
	celExpressionAccessor := make([]cel.ExpressionAccessor, len(inputValidations))
	for i, validation := range inputValidations {
		validation := validating.ValidationCondition{
			Expression: validation.Expression,
			Message:    validation.Message,
			Reason:     validation.Reason,
		}
		celExpressionAccessor[i] = &validation
	}
	return celExpressionAccessor
}

func convertv1MessageExpressions(inputValidations []v1.Validation) []cel.ExpressionAccessor {
	celExpressionAccessor := make([]cel.ExpressionAccessor, len(inputValidations))
	for i, validation := range inputValidations {
		if validation.MessageExpression != "" {
			condition := validating.MessageExpressionCondition{
				MessageExpression: validation.MessageExpression,
			}
			celExpressionAccessor[i] = &condition
		}
	}
	return celExpressionAccessor
}

func convertv1AuditAnnotations(inputValidations []v1.AuditAnnotation) []cel.ExpressionAccessor {
	celExpressionAccessor := make([]cel.ExpressionAccessor, len(inputValidations))
	for i, validation := range inputValidations {
		validation := validating.AuditAnnotationCondition{
			Key:             validation.Key,
			ValueExpression: validation.ValueExpression,
		}
		celExpressionAccessor[i] = &validation
	}
	return celExpressionAccessor
}

func convertv1beta1Variables(variables []v1.Variable) []cel.NamedExpressionAccessor {
	namedExpressions := make([]cel.NamedExpressionAccessor, len(variables))
	for i, variable := range variables {
		namedExpressions[i] = &validating.Variable{Name: variable.Name, Expression: variable.Expression}
	}
	return namedExpressions
}
