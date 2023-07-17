/*
   Copyright The Soci Snapshotter Authors.

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

/*
   Copyright The containerd Authors.

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

package local_keychain

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/awslabs/soci-snapshotter/proto"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/reference"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"
)

var (
	localKeychainPort = flag.Int("local_keychain_port", 0,
		"Port on which to expose the local_keychain gRPC service that accepts username/password credentials for private images. If 0, the local_keychain service is not started/exposed.")
)

type credentials struct {
	username   string
	password   string
	expiration *time.Time
}

type keychain struct {
	mu    sync.Mutex
	cache map[string]credentials
	proto.UnimplementedLocalKeychainServer
}

var singleton *keychain
var lock = &sync.Mutex{}

func (kc *keychain) init() {
	if *localKeychainPort == 0 {
		log.G(context.Background()).Info("no local_keychain_port specified, not starting local_keychain gRPC server")
		return
	}

	lis, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", *localKeychainPort))
	if err != nil {
		log.G(context.Background()).Fatalf("failed to listen: %v", err)
	} else {
		log.G(context.Background()).Infof("started local keychain server on localhost:%d", *localKeychainPort)
	}
	// Set to avoid errors: Bandwidth exhausted HTTP/2 error code: ENHANCE_YOUR_CALM Received Goaway too_many_pings
	opts := []grpc.ServerOption{
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             10 * time.Second, // If a client pings more than once every 10 seconds, terminate the connection
			PermitWithoutStream: true,             // Allow pings even when there are no active streams
		})}
	grpcServer := grpc.NewServer(opts...)
	proto.RegisterLocalKeychainServer(grpcServer, kc)
	reflection.Register(grpcServer)
	go grpcServer.Serve(lis)
}

func (kc *keychain) PutCredentials(ctx context.Context, req *proto.PutCredentialsRequest) (res *proto.PutCredentialsResponse, err error) {
	if req.ImageName != "" && req.Credentials != nil {
		log.G(ctx).Infof("received credentials for image %s, caching for %d seconds", req.ImageName, req.ExpiresInSeconds)
		var expirationTime *time.Time
		if req.ExpiresInSeconds > 0 {
			timeout := time.Now().Add(time.Duration(req.ExpiresInSeconds) * time.Second)
			expirationTime = &timeout
		}
		kc.mu.Lock()
		defer kc.mu.Unlock()
		kc.cache[req.ImageName] = credentials{
			username:   req.Credentials.Username,
			password:   req.Credentials.Password,
			expiration: expirationTime,
		}
	}
	return &proto.PutCredentialsResponse{}, nil
}

func (kc *keychain) GetCredentials(host string, refspec reference.Spec) (string, string, error) {
	kc.mu.Lock()
	defer kc.mu.Unlock()
	creds, found := kc.cache[refspec.String()]
	if found && (creds.expiration == nil || creds.expiration.After(time.Now())) {
		return creds.username, creds.password, nil
	} else if found {
		// Credentials were cached but have expired, remove them.
		delete(kc.cache, refspec.String())
	}
	return "", "", errors.New("credentials not found")
}

func Keychain(ctx context.Context) *keychain {
	lock.Lock()
	defer lock.Unlock()
	if singleton == nil {
		singleton = &keychain{
			cache: map[string]credentials{},
		}
		singleton.init()
	}
	return singleton
}
