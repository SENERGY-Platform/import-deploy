/*
 * Copyright 2021 InfAI (CC SES)
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

package util

import (
	"context"
	"time"
)

// GetTimeoutContext returns a context for a single call to a backend. It keeps the
// values of ctx -- the trace and the baggage of the request that caused the call --
// and puts the usual timeout on it.
func GetTimeoutContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, 10*time.Second)
}

// WriteContext keeps the values of ctx but drops its cancellation.
//
// Creating, updating and deleting an instance each write to the deployment backend
// and to the database, and the two have to agree. A request cancelled between the
// two writes would leave a container running that no instance points at, or an
// instance whose container is gone -- which the startup restore then recreates. The
// read paths stay cancellable.
func WriteContext(ctx context.Context) context.Context {
	return context.WithoutCancel(ctx)
}
