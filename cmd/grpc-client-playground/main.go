/*
 * Copyright 2024 Galactica Network
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package main

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/Galactica-corp/merkle-proof-service/gen/galactica/merkle"
)

func main() {
	// GRPC client for merkle proof service
	//url := "grpc-merkle-proof-service.galactica.com:443"
	url := "localhost:50651"

	// Create a new connection
	conn, err := grpc.Dial(url, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		panic(err)
	}

	// Close the connection
	defer conn.Close()

	// Create a new client
	client := merkle.NewQueryClient(conn)
	resp, err := client.GetEmptyLeafProof(context.Background(), &merkle.GetEmptyLeafProofRequest{
		Registry: "0xbc196948e8c1Bc416aEaCf309a63DCEFfdf0cE31",
	})

	if err != nil {
		panic(err)
	}

	// Print the response
	fmt.Println(resp)
}
