package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/cycl0o0/GPTerminal/internal/memory"
	"github.com/spf13/cobra"
)

var memoryCmd = &cobra.Command{
	Use:   "memory",
	Short: "Manage persistent conversation memory",
	Long:  "List, set, delete, or clear memories that persist across chat sessions.",
}

var memoryListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all saved memories",
	Run: func(cmd *cobra.Command, args []string) {
		store, err := memory.Load()
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}
		fmt.Print(store.List())
	},
}

var memorySetCmd = &cobra.Command{
	Use:   "set <key> <value...>",
	Short: "Save a memory",
	Args:  cobra.MinimumNArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		store, err := memory.Load()
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}
		key := args[0]
		value := strings.Join(args[1:], " ")
		store.Set(key, value)
		if err := store.Save(); err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}
		fmt.Printf("Saved: %s = %s\n", key, value)
	},
}

var memoryDeleteCmd = &cobra.Command{
	Use:   "delete <key>",
	Short: "Delete a memory",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		store, err := memory.Load()
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}
		if store.Delete(args[0]) {
			if err := store.Save(); err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err)
				os.Exit(1)
			}
			fmt.Printf("Deleted: %s\n", args[0])
		} else {
			fmt.Printf("Not found: %s\n", args[0])
		}
	},
}

var memoryClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clear all memories",
	Run: func(cmd *cobra.Command, args []string) {
		store := &memory.Store{}
		if err := store.Save(); err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}
		fmt.Println("All memories cleared.")
	},
}

var memorySearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search memories",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		store, err := memory.Load()
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}
		results := store.Search(args[0])
		if len(results) == 0 {
			fmt.Println("No matching memories found.")
			return
		}
		for _, e := range results {
			fmt.Printf("- %s: %s\n", e.Key, e.Value)
		}
	},
}

func init() {
	memoryCmd.AddCommand(memoryListCmd)
	memoryCmd.AddCommand(memorySetCmd)
	memoryCmd.AddCommand(memoryDeleteCmd)
	memoryCmd.AddCommand(memoryClearCmd)
	memoryCmd.AddCommand(memorySearchCmd)
	rootCmd.AddCommand(memoryCmd)
}
