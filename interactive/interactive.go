// interactive/interactive.go
package interactive

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/sammcj/gomcp/bridge"
	"github.com/sammcj/gomcp/config"
)

type Interactive struct {
    logger  *log.Logger
    scanner *bufio.Reader
    cfg     *config.Config
    bridge  *bridge.Bridge
    debug   bool
}

func New(cfg *config.Config) *Interactive {
    return &Interactive{
        scanner: bufio.NewReader(os.Stdin),
        logger:  log.Default(),
        cfg:     cfg,
        debug:   strings.ToLower(cfg.Logging.Level) == "debug",
    }
}

func (i *Interactive) Start() error {
    // Initialize bridge
    var err error
    i.bridge, err = bridge.New(i.cfg, i.logger)
    if err != nil {
        return fmt.Errorf("failed to create bridge: %w", err)
    }
    defer i.bridge.Close()

    // Initialize bridge components
    if err := i.bridge.Initialize(); err != nil {
        return fmt.Errorf("failed to initialize bridge: %w", err)
    }

    fmt.Println("\n=== Ollama Chat Interface Ready ===")
    fmt.Println("Type 'quit' or press Ctrl+C to exit")
    fmt.Println("Connected to model:", i.cfg.LLM.Model)
    fmt.Printf("Using endpoint: %s\n", i.cfg.LLM.Endpoint)
    fmt.Println("Database:", i.cfg.Database.Path)
    fmt.Println("================================")

    for {
        fmt.Print("\nEnter your message: ")
        input, err := i.scanner.ReadString('\n')
        if err != nil {
            if i.debug {
                i.logger.Printf("Error reading input: %v", err)
            }
            continue
        }

        input = strings.TrimSpace(input)
        if input == "" {
            continue
        }

        if input == "quit" || input == "exit" {
            fmt.Println("Goodbye!")
            return nil
        }

        // Process message through bridge
        if i.debug {
            i.logger.Printf("Sending message to bridge: %s", input)
        }
        response, err := i.bridge.ProcessMessage(input)
        if err != nil {
            if i.debug {
                i.logger.Printf("Error from bridge: %v", err)
            }
            fmt.Printf("\nError: %v\n", err)
            continue
        }

        if response == "" {
            if i.debug {
                i.logger.Printf("Warning: Empty response received from bridge")
            }
            fmt.Println("\nNo response received.")
            continue
        }

        // Print the response with a newline before and after for better readability
        fmt.Printf("\n%s\n", response)
    }
}

func (i *Interactive) Shutdown() error {
    if i.bridge != nil {
        return i.bridge.Close()
    }
    return nil
}
