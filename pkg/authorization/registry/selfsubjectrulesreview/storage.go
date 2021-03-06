package selfsubjectrulesreview

import (
	"fmt"
	"sort"

	kapi "k8s.io/kubernetes/pkg/api"
	kapierrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/runtime"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/authorization/authorizer/scope"
	"github.com/openshift/origin/pkg/authorization/rulevalidation"
)

type REST struct {
	ruleResolver        rulevalidation.AuthorizationRuleResolver
	clusterPolicyGetter rulevalidation.ClusterPolicyGetter
}

func NewREST(ruleResolver rulevalidation.AuthorizationRuleResolver, clusterPolicyGetter rulevalidation.ClusterPolicyGetter) *REST {
	return &REST{ruleResolver: ruleResolver, clusterPolicyGetter: clusterPolicyGetter}
}

func (r *REST) New() runtime.Object {
	return &authorizationapi.SelfSubjectRulesReview{}
}

// Create registers a given new ResourceAccessReview instance to r.registry.
func (r *REST) Create(ctx kapi.Context, obj runtime.Object) (runtime.Object, error) {
	rulesReview, ok := obj.(*authorizationapi.SelfSubjectRulesReview)
	if !ok {
		return nil, kapierrors.NewBadRequest(fmt.Sprintf("not a SelfSubjectRulesReview: %#v", obj))
	}
	namespace := kapi.NamespaceValue(ctx)
	if len(namespace) == 0 {
		return nil, kapierrors.NewBadRequest(fmt.Sprintf("namespace is required on this type: %v", namespace))
	}
	user, exists := kapi.UserFrom(ctx)
	if !exists {
		return nil, fmt.Errorf("user missing from context")
	}

	policyRules, err := r.ruleResolver.GetEffectivePolicyRules(ctx)
	rules := []authorizationapi.PolicyRule{}
	for _, rule := range policyRules {
		rules = append(rules, rulevalidation.BreakdownRule(rule)...)
	}

	switch {
	case rulesReview.Spec.Scopes == nil:
		if scopes, _ := user.GetExtra()[authorizationapi.ScopesKey]; len(scopes) > 0 {
			rules, err = r.filterRulesByScopes(rules, scopes, namespace)
			if err != nil {
				return nil, err
			}
		}

	case len(rulesReview.Spec.Scopes) > 0:
		rules, err = r.filterRulesByScopes(rules, rulesReview.Spec.Scopes, namespace)
		if err != nil {
			return nil, err
		}

	}

	if compactedRules, err := rulevalidation.CompactRules(rules); err == nil {
		rules = compactedRules
	}
	sort.Sort(authorizationapi.SortableRuleSlice(rules))

	ret := &authorizationapi.SelfSubjectRulesReview{
		Status: authorizationapi.SubjectRulesReviewStatus{
			Rules: rules,
		},
	}

	if err != nil {
		ret.Status.EvaluationError = err.Error()
	}

	return ret, nil
}

func (r *REST) filterRulesByScopes(rules []authorizationapi.PolicyRule, scopes []string, namespace string) ([]authorizationapi.PolicyRule, error) {
	scopeRules, err := scope.ScopesToRules(scopes, namespace, r.clusterPolicyGetter)
	if err != nil {
		return nil, err
	}

	filteredRules := []authorizationapi.PolicyRule{}
	for _, rule := range rules {
		if allowed, _ := rulevalidation.Covers(scopeRules, []authorizationapi.PolicyRule{rule}); allowed {
			filteredRules = append(filteredRules, rule)
		}
	}

	return filteredRules, nil
}
