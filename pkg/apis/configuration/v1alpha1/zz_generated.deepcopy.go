//go:build !ignore_autogenerated
// +build !ignore_autogenerated

/*
Copyright The Dapr Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Code generated by controller-gen. DO NOT EDIT.

package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *APIAccessRule) DeepCopyInto(out *APIAccessRule) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new APIAccessRule.
func (in *APIAccessRule) DeepCopy() *APIAccessRule {
	if in == nil {
		return nil
	}
	out := new(APIAccessRule)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *APILoggingSpec) DeepCopyInto(out *APILoggingSpec) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new APILoggingSpec.
func (in *APILoggingSpec) DeepCopy() *APILoggingSpec {
	if in == nil {
		return nil
	}
	out := new(APILoggingSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *APISpec) DeepCopyInto(out *APISpec) {
	*out = *in
	if in.Allowed != nil {
		in, out := &in.Allowed, &out.Allowed
		*out = make([]APIAccessRule, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new APISpec.
func (in *APISpec) DeepCopy() *APISpec {
	if in == nil {
		return nil
	}
	out := new(APISpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AccessControlSpec) DeepCopyInto(out *AccessControlSpec) {
	*out = *in
	if in.AppPolicies != nil {
		in, out := &in.AppPolicies, &out.AppPolicies
		*out = make([]AppPolicySpec, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AccessControlSpec.
func (in *AccessControlSpec) DeepCopy() *AccessControlSpec {
	if in == nil {
		return nil
	}
	out := new(AccessControlSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AppOperationAction) DeepCopyInto(out *AppOperationAction) {
	*out = *in
	if in.HTTPVerb != nil {
		in, out := &in.HTTPVerb, &out.HTTPVerb
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AppOperationAction.
func (in *AppOperationAction) DeepCopy() *AppOperationAction {
	if in == nil {
		return nil
	}
	out := new(AppOperationAction)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AppPolicySpec) DeepCopyInto(out *AppPolicySpec) {
	*out = *in
	if in.AppOperationActions != nil {
		in, out := &in.AppOperationActions, &out.AppOperationActions
		*out = make([]AppOperationAction, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AppPolicySpec.
func (in *AppPolicySpec) DeepCopy() *AppPolicySpec {
	if in == nil {
		return nil
	}
	out := new(AppPolicySpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ComponentsSpec) DeepCopyInto(out *ComponentsSpec) {
	*out = *in
	if in.Deny != nil {
		in, out := &in.Deny, &out.Deny
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ComponentsSpec.
func (in *ComponentsSpec) DeepCopy() *ComponentsSpec {
	if in == nil {
		return nil
	}
	out := new(ComponentsSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Configuration) DeepCopyInto(out *Configuration) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Configuration.
func (in *Configuration) DeepCopy() *Configuration {
	if in == nil {
		return nil
	}
	out := new(Configuration)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *Configuration) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ConfigurationList) DeepCopyInto(out *ConfigurationList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]Configuration, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ConfigurationList.
func (in *ConfigurationList) DeepCopy() *ConfigurationList {
	if in == nil {
		return nil
	}
	out := new(ConfigurationList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ConfigurationList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ConfigurationSpec) DeepCopyInto(out *ConfigurationSpec) {
	*out = *in
	if in.AppHTTPPipelineSpec != nil {
		in, out := &in.AppHTTPPipelineSpec, &out.AppHTTPPipelineSpec
		*out = new(PipelineSpec)
		(*in).DeepCopyInto(*out)
	}
	if in.HTTPPipelineSpec != nil {
		in, out := &in.HTTPPipelineSpec, &out.HTTPPipelineSpec
		*out = new(PipelineSpec)
		(*in).DeepCopyInto(*out)
	}
	if in.TracingSpec != nil {
		in, out := &in.TracingSpec, &out.TracingSpec
		*out = new(TracingSpec)
		(*in).DeepCopyInto(*out)
	}
	if in.MetricSpec != nil {
		in, out := &in.MetricSpec, &out.MetricSpec
		*out = new(MetricSpec)
		(*in).DeepCopyInto(*out)
	}
	if in.MetricsSpec != nil {
		in, out := &in.MetricsSpec, &out.MetricsSpec
		*out = new(MetricSpec)
		(*in).DeepCopyInto(*out)
	}
	if in.MTLSSpec != nil {
		in, out := &in.MTLSSpec, &out.MTLSSpec
		*out = new(MTLSSpec)
		**out = **in
	}
	if in.Secrets != nil {
		in, out := &in.Secrets, &out.Secrets
		*out = new(SecretsSpec)
		(*in).DeepCopyInto(*out)
	}
	if in.AccessControlSpec != nil {
		in, out := &in.AccessControlSpec, &out.AccessControlSpec
		*out = new(AccessControlSpec)
		(*in).DeepCopyInto(*out)
	}
	if in.NameResolutionSpec != nil {
		in, out := &in.NameResolutionSpec, &out.NameResolutionSpec
		*out = new(NameResolutionSpec)
		(*in).DeepCopyInto(*out)
	}
	if in.Features != nil {
		in, out := &in.Features, &out.Features
		*out = make([]FeatureSpec, len(*in))
		copy(*out, *in)
	}
	if in.APISpec != nil {
		in, out := &in.APISpec, &out.APISpec
		*out = new(APISpec)
		(*in).DeepCopyInto(*out)
	}
	if in.ComponentsSpec != nil {
		in, out := &in.ComponentsSpec, &out.ComponentsSpec
		*out = new(ComponentsSpec)
		(*in).DeepCopyInto(*out)
	}
	if in.LoggingSpec != nil {
		in, out := &in.LoggingSpec, &out.LoggingSpec
		*out = new(LoggingSpec)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ConfigurationSpec.
func (in *ConfigurationSpec) DeepCopy() *ConfigurationSpec {
	if in == nil {
		return nil
	}
	out := new(ConfigurationSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DynamicValue) DeepCopyInto(out *DynamicValue) {
	*out = *in
	in.JSON.DeepCopyInto(&out.JSON)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DynamicValue.
func (in *DynamicValue) DeepCopy() *DynamicValue {
	if in == nil {
		return nil
	}
	out := new(DynamicValue)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *FeatureSpec) DeepCopyInto(out *FeatureSpec) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new FeatureSpec.
func (in *FeatureSpec) DeepCopy() *FeatureSpec {
	if in == nil {
		return nil
	}
	out := new(FeatureSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *HandlerSpec) DeepCopyInto(out *HandlerSpec) {
	*out = *in
	if in.SelectorSpec != nil {
		in, out := &in.SelectorSpec, &out.SelectorSpec
		*out = new(SelectorSpec)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new HandlerSpec.
func (in *HandlerSpec) DeepCopy() *HandlerSpec {
	if in == nil {
		return nil
	}
	out := new(HandlerSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LoggingSpec) DeepCopyInto(out *LoggingSpec) {
	*out = *in
	if in.APILogging != nil {
		in, out := &in.APILogging, &out.APILogging
		*out = new(APILoggingSpec)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LoggingSpec.
func (in *LoggingSpec) DeepCopy() *LoggingSpec {
	if in == nil {
		return nil
	}
	out := new(LoggingSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MTLSSpec) DeepCopyInto(out *MTLSSpec) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MTLSSpec.
func (in *MTLSSpec) DeepCopy() *MTLSSpec {
	if in == nil {
		return nil
	}
	out := new(MTLSSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MetricLabel) DeepCopyInto(out *MetricLabel) {
	*out = *in
	if in.Regex != nil {
		in, out := &in.Regex, &out.Regex
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MetricLabel.
func (in *MetricLabel) DeepCopy() *MetricLabel {
	if in == nil {
		return nil
	}
	out := new(MetricLabel)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MetricSpec) DeepCopyInto(out *MetricSpec) {
	*out = *in
	if in.Rules != nil {
		in, out := &in.Rules, &out.Rules
		*out = make([]MetricsRule, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MetricSpec.
func (in *MetricSpec) DeepCopy() *MetricSpec {
	if in == nil {
		return nil
	}
	out := new(MetricSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MetricsRule) DeepCopyInto(out *MetricsRule) {
	*out = *in
	if in.Labels != nil {
		in, out := &in.Labels, &out.Labels
		*out = make([]MetricLabel, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MetricsRule.
func (in *MetricsRule) DeepCopy() *MetricsRule {
	if in == nil {
		return nil
	}
	out := new(MetricsRule)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NameResolutionSpec) DeepCopyInto(out *NameResolutionSpec) {
	*out = *in
	if in.Configuration != nil {
		in, out := &in.Configuration, &out.Configuration
		*out = new(DynamicValue)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NameResolutionSpec.
func (in *NameResolutionSpec) DeepCopy() *NameResolutionSpec {
	if in == nil {
		return nil
	}
	out := new(NameResolutionSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *OtelSpec) DeepCopyInto(out *OtelSpec) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new OtelSpec.
func (in *OtelSpec) DeepCopy() *OtelSpec {
	if in == nil {
		return nil
	}
	out := new(OtelSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PipelineSpec) DeepCopyInto(out *PipelineSpec) {
	*out = *in
	if in.Handlers != nil {
		in, out := &in.Handlers, &out.Handlers
		*out = make([]HandlerSpec, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PipelineSpec.
func (in *PipelineSpec) DeepCopy() *PipelineSpec {
	if in == nil {
		return nil
	}
	out := new(PipelineSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SecretsScope) DeepCopyInto(out *SecretsScope) {
	*out = *in
	if in.AllowedSecrets != nil {
		in, out := &in.AllowedSecrets, &out.AllowedSecrets
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.DeniedSecrets != nil {
		in, out := &in.DeniedSecrets, &out.DeniedSecrets
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SecretsScope.
func (in *SecretsScope) DeepCopy() *SecretsScope {
	if in == nil {
		return nil
	}
	out := new(SecretsScope)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SecretsSpec) DeepCopyInto(out *SecretsSpec) {
	*out = *in
	if in.Scopes != nil {
		in, out := &in.Scopes, &out.Scopes
		*out = make([]SecretsScope, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SecretsSpec.
func (in *SecretsSpec) DeepCopy() *SecretsSpec {
	if in == nil {
		return nil
	}
	out := new(SecretsSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SelectorField) DeepCopyInto(out *SelectorField) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SelectorField.
func (in *SelectorField) DeepCopy() *SelectorField {
	if in == nil {
		return nil
	}
	out := new(SelectorField)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SelectorSpec) DeepCopyInto(out *SelectorSpec) {
	*out = *in
	if in.Fields != nil {
		in, out := &in.Fields, &out.Fields
		*out = make([]SelectorField, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SelectorSpec.
func (in *SelectorSpec) DeepCopy() *SelectorSpec {
	if in == nil {
		return nil
	}
	out := new(SelectorSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TracingSpec) DeepCopyInto(out *TracingSpec) {
	*out = *in
	if in.Zipkin != nil {
		in, out := &in.Zipkin, &out.Zipkin
		*out = new(ZipkinSpec)
		**out = **in
	}
	if in.Otel != nil {
		in, out := &in.Otel, &out.Otel
		*out = new(OtelSpec)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TracingSpec.
func (in *TracingSpec) DeepCopy() *TracingSpec {
	if in == nil {
		return nil
	}
	out := new(TracingSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ZipkinSpec) DeepCopyInto(out *ZipkinSpec) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ZipkinSpec.
func (in *ZipkinSpec) DeepCopy() *ZipkinSpec {
	if in == nil {
		return nil
	}
	out := new(ZipkinSpec)
	in.DeepCopyInto(out)
	return out
}
