package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"os"
	"time"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/joho/godotenv"
	"github.com/rss3-network/node/provider/arweave"
)

type Network struct {
	Name string
	URL  string
	Type string
}

type Config struct {
	NetworkStartBlock map[string]int64 `json:"network_start_block"`
}

func findClosestBlockRPC(rpcClient *rpc.Client, targetTimestamp int64) (*big.Int, error) {
	ctx := context.Background()

	var result hexutil.Big
	err := rpcClient.CallContext(ctx, &result, "eth_blockNumber")
	if err != nil {
		return nil, fmt.Errorf("error getting latest block number: %v", err)
	}
	high := (*big.Int)(&result)

	low := big.NewInt(1)

	for low.Cmp(high) <= 0 {
		mid := new(big.Int).Add(low, high)
		mid.Div(mid, big.NewInt(2))

		var block struct {
			Timestamp string `json:"timestamp"`
		}
		err := rpcClient.CallContext(ctx, &block, "eth_getBlockByNumber", hexutil.EncodeBig(mid), false)
		if err != nil {
			return nil, fmt.Errorf("error getting block %s: %v", mid.String(), err)
		}

		blockTimestamp, _ := hexutil.DecodeBig(block.Timestamp)
		if blockTimestamp.Int64() == targetTimestamp {
			return mid, nil
		} else if blockTimestamp.Int64() < targetTimestamp {
			low = new(big.Int).Add(mid, big.NewInt(1))
		} else {
			high = new(big.Int).Sub(mid, big.NewInt(1))
		}
	}

	return low, nil
}

func findClosestBlockArweave(client arweave.Client, targetTimestamp int64) (int64, error) {
	ctx := context.Background()

	high, err := client.GetBlockHeight(ctx)
	if err != nil {
		return 0, fmt.Errorf("error getting latest block height: %v", err)
	}

	low := int64(1)

	for low <= high {
		mid := (low + high) / 2

		block, err := client.GetBlockByHeight(ctx, mid)
		if err != nil {
			return 0, fmt.Errorf("error getting block %d: %v", mid, err)
		}

		if block.Timestamp == targetTimestamp {
			return mid, nil
		} else if block.Timestamp < targetTimestamp {
			low = mid + 1
		} else {
			high = mid - 1
		}
	}

	return low, nil
}

func main() {
	targetTimestamp := int64(1717200000)

	// Load .env file
	err := godotenv.Load()
	if err != nil {
		log.Printf("Error loading .env file: %v", err)
		// Continue execution even if .env file is not found
	}

	// Read config.json
	configFile, err := os.ReadFile("config.json")
	if err != nil {
		log.Fatalf("Error reading config file: %v", err)
	}

	var config Config
	err = json.Unmarshal(configFile, &config)
	if err != nil {
		log.Fatalf("Error parsing config file: %v", err)
	}

	fmt.Println("Network start blocks from config:")
	for network, block := range config.NetworkStartBlock {
		fmt.Printf("%s: %d\n", network, block)
	}
	fmt.Println()

	networks := []Network{
		{"ethereum", os.Getenv("ETHEREUM_RPC_URL"), "ethereum"},
		{"polygon", os.Getenv("POLYGON_RPC_URL"), "ethereum"},
		{"avax", os.Getenv("AVALANCHE_RPC_URL"), "ethereum"},
		{"optimism", os.Getenv("OPTIMISM_RPC_URL"), "ethereum"},
		{"arbitrum", os.Getenv("ARBITRUM_RPC_URL"), "ethereum"},
		{"gnosis", os.Getenv("GNOSIS_RPC_URL"), "ethereum"},
		{"linea", os.Getenv("LINEA_RPC_URL"), "ethereum"},
		{"binance-smart-chain", os.Getenv("BSC_RPC_URL"), "ethereum"},
		{"base", os.Getenv("BASE_RPC_URL"), "ethereum"},
		{"crossbell", os.Getenv("CROSSBELL_RPC_URL"), "ethereum"},
		{"vsl", os.Getenv("VSL_RPC_URL"), "ethereum"},
		{"x-layer", os.Getenv("XLAYER_RPC_URL"), "ethereum"},
		{"arweave", os.Getenv("ARWEAVE_RPC_URL"), "arweave"},
	}

	for _, network := range networks {
		fmt.Printf("Network: %s\n", network.Name)

		var closestBlockInt64 int64

		if network.Type == "ethereum" {
			rpcClient, err := rpc.Dial(network.URL)
			if err != nil {
				log.Printf("Error connecting to %s: %v\n", network.Name, err)
				fmt.Println()
				continue
			}
			defer rpcClient.Close()

			// Try to get the latest block to check if the network is responsive
			var latestBlock map[string]interface{}
			err = rpcClient.CallContext(context.Background(), &latestBlock, "eth_getBlockByNumber", "latest", false)
			if err != nil {
				log.Printf("Error getting latest block from %s: %v\n", network.Name, err)
				fmt.Println()
				continue
			}

			closestBlock, err := findClosestBlockRPC(rpcClient, targetTimestamp)
			if err != nil {
				log.Printf("Error finding closest block for %s: %v\n", network.Name, err)
				fmt.Println()
				continue
			}

			var block struct {
				Timestamp string `json:"timestamp"`
			}
			err = rpcClient.CallContext(context.Background(), &block, "eth_getBlockByNumber", hexutil.EncodeBig(closestBlock), false)
			if err != nil {
				log.Printf("Error getting block details for %s: %v\n", network.Name, err)
				fmt.Println()
				continue
			}

			blockTimestamp, _ := hexutil.DecodeBig(block.Timestamp)

			fmt.Printf("Closest block number: %s\n", closestBlock.String())
			fmt.Printf("Block timestamp: %s\n", time.Unix(blockTimestamp.Int64(), 0))
			fmt.Printf("Difference from target: %d seconds\n", blockTimestamp.Int64()-targetTimestamp)

			closestBlockInt64 = closestBlock.Int64()

		} else if network.Type == "arweave" {
			arweaveClient, err := arweave.NewClient(arweave.WithGateways([]string{network.URL}))
			if err != nil {
				log.Printf("Error creating Arweave client for %s: %v\n", network.Name, err)
				fmt.Println()
				continue
			}

			closestBlock, err := findClosestBlockArweave(arweaveClient, targetTimestamp)
			if err != nil {
				log.Printf("Error finding closest block for %s: %v\n", network.Name, err)
				fmt.Println()
				continue
			}

			block, err := arweaveClient.GetBlockByHeight(context.Background(), closestBlock)
			if err != nil {
				log.Printf("Error getting block details for %s: %v\n", network.Name, err)
				fmt.Println()
				continue
			}

			fmt.Printf("Closest block number: %d\n", closestBlock)
			fmt.Printf("Block timestamp: %s\n", time.Unix(block.Timestamp, 0))
			fmt.Printf("Difference from target: %d seconds\n", block.Timestamp-targetTimestamp)

			closestBlockInt64 = closestBlock
		}

		// Update config with new value
		config.NetworkStartBlock[network.Name] = closestBlockInt64
		fmt.Printf("Updated start block for %s: %d\n", network.Name, closestBlockInt64)
		fmt.Println()
	}

	// Update Farcaster timestamp
	farcasterTimestamp := targetTimestamp - (9 * 30 * 24 * 60 * 60) // Subtract 9 months (approx.)
	config.NetworkStartBlock["farcaster"] = farcasterTimestamp
	fmt.Printf("Updated start block for farcaster: %d\n", farcasterTimestamp)
	fmt.Println()

	// Write updated config back to file
	updatedConfig, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		log.Fatalf("Error marshaling updated config: %v", err)
	}

	err = os.WriteFile("config.json", updatedConfig, 0644)
	if err != nil {
		// If writing fails, try to retry a few times
		for i := 0; i < 3; i++ {
			time.Sleep(time.Second) // Wait for a second before retrying
			err = os.WriteFile("config.json", updatedConfig, 0644)
			if err == nil {
				break
			}
		}
		if err != nil {
			log.Fatalf("Error writing updated config file after retries: %v", err)
		}
	}

	fmt.Println("Config file updated successfully.")
}
