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

package shared

import ()

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
func (in *NameValuePair) DeepCopyInto(out *NameValuePair) {
	*out = *in
	in.Value.DeepCopyInto(&out.Value)
	out.SecretKeyRef = in.SecretKeyRef
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NameValuePair.
func (in *NameValuePair) DeepCopy() *NameValuePair {
	if in == nil {
		return nil
	}
	out := new(NameValuePair)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Scoped) DeepCopyInto(out *Scoped) {
	*out = *in
	if in.Scopes != nil {
		in, out := &in.Scopes, &out.Scopes
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Scoped.
func (in *Scoped) DeepCopy() *Scoped {
	if in == nil {
		return nil
	}
	out := new(Scoped)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SecretKeyRef) DeepCopyInto(out *SecretKeyRef) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SecretKeyRef.
func (in *SecretKeyRef) DeepCopy() *SecretKeyRef {
	if in == nil {
		return nil
	}
	out := new(SecretKeyRef)
	in.DeepCopyInto(out)
	return out
}
