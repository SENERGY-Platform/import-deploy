/*
 * Copyright 2020 InfAI (CC SES)
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *    http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package model

import "github.com/SENERGY-Platform/models/go/models"

// An import type belongs to the import repository; its types live in the shared model,
// which is where the import repository defines them too. They are aliased rather than
// imported directly, because the controller of this service is compiled against these
// names.
//
// The shared type also carries the fields this service does not read itself -- the
// output description with its aspects and functions, and the cost -- which the local
// copy used to drop on decode.

type ImportType = models.ImportType

// ImportTypeConfig declares a config of an import type. It is the counterpart of
// InstanceConfig, which holds the value an instance actually runs with.
type ImportTypeConfig = models.ImportTypeConfig

// ContentVariable describes the output of an import type. It carries AspectIds; the
// deprecated AspectId is an alias for a single entry of that list. Both are filled by
// the import repository on read, so a reader here has to interpret only one of them and
// needs no folding of its own.
type ContentVariable = models.ImportContentVariable

type Type = models.Type

const (
	String  = models.String
	Integer = models.Integer
	Float   = models.Float
	Boolean = models.Boolean

	List      = models.List
	Structure = models.Structure
)
