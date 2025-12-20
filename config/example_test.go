package config_test

import (
	"fmt"
	"os"
	"time"

	"github.com/Ranganaths/minion/config"
)

func ExampleEnv_GetString() {
	// Create an env helper with a prefix
	env := config.NewEnv("MYAPP")

	// Set an environment variable for the example
	os.Setenv("MYAPP_DATABASE_HOST", "localhost")
	defer os.Unsetenv("MYAPP_DATABASE_HOST")

	// Get the value with a default
	host := env.GetString("DATABASE_HOST", "127.0.0.1")
	fmt.Println(host)
	// Output: localhost
}

func ExampleEnv_GetInt() {
	env := config.NewEnv("MYAPP")

	os.Setenv("MYAPP_PORT", "8080")
	defer os.Unsetenv("MYAPP_PORT")

	port := env.GetInt("PORT", 3000)
	fmt.Println(port)
	// Output: 8080
}

func ExampleEnv_GetBool() {
	env := config.NewEnv("MYAPP")

	os.Setenv("MYAPP_DEBUG", "true")
	defer os.Unsetenv("MYAPP_DEBUG")

	debug := env.GetBool("DEBUG", false)
	fmt.Println(debug)
	// Output: true
}

func ExampleEnv_GetDuration() {
	env := config.NewEnv("MYAPP")

	os.Setenv("MYAPP_TIMEOUT", "30s")
	defer os.Unsetenv("MYAPP_TIMEOUT")

	timeout := env.GetDuration("TIMEOUT", 10*time.Second)
	fmt.Println(timeout)
	// Output: 30s
}

func ExampleEnv_GetStringSlice() {
	env := config.NewEnv("MYAPP")

	os.Setenv("MYAPP_HOSTS", "host1, host2, host3")
	defer os.Unsetenv("MYAPP_HOSTS")

	hosts := env.GetStringSlice("HOSTS", []string{"localhost"})
	fmt.Printf("Hosts: %v\n", hosts)
	// Output: Hosts: [host1 host2 host3]
}

func ExampleGetString() {
	// Using the default MINION prefix
	os.Setenv("MINION_LLM_PROVIDER", "anthropic")
	defer os.Unsetenv("MINION_LLM_PROVIDER")

	provider := config.GetString("LLM_PROVIDER", "openai")
	fmt.Println(provider)
	// Output: anthropic
}
