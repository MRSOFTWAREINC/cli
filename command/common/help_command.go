package common

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"code.cloudfoundry.org/cli/actor/sharedaction"
	"code.cloudfoundry.org/cli/command"
	"code.cloudfoundry.org/cli/command/common/internal"
	"code.cloudfoundry.org/cli/command/flag"
	"code.cloudfoundry.org/cli/util/configv3"
	"code.cloudfoundry.org/cli/util/sorting"
)

const (
	commonCommandsIndent string = "  "
	allCommandsIndent    string = "   "
	commandIndent        string = "   "
)

//go:generate counterfeiter . HelpActor

// HelpActor handles the business logic of the help command
type HelpActor interface {
	// CommandInfoByName returns back a help command information for the given
	// command
	CommandInfoByName(interface{}, string) (sharedaction.CommandInfo, error)

	// CommandInfos returns a list of all commands
	CommandInfos(interface{}) map[string]sharedaction.CommandInfo
}

type HelpCommand struct {
	UI     command.UI
	Actor  HelpActor
	Config command.Config

	OptionalArgs flag.CommandName `positional-args:"yes"`
	AllCommands  bool             `short:"a" description:"All available CLI commands"`
	usage        interface{}      `usage:"CF_NAME help [COMMAND]"`
}

func (cmd *HelpCommand) Setup(config command.Config, ui command.UI) error {
	cmd.Actor = sharedaction.NewActor()
	cmd.Config = config
	cmd.UI = ui

	return nil
}

func (cmd HelpCommand) Execute(args []string) error {
	var err error
	if cmd.OptionalArgs.CommandName == "" {
		cmd.displayFullHelp()
	} else {
		err = cmd.displayCommand()
	}

	return err
}

func (cmd HelpCommand) displayFullHelp() {
	if cmd.AllCommands {
		cmd.displayHelpPreamble()
		cmd.displayAllCommands()
		cmd.displayHelpFooter()
	} else {
		cmd.displayCommonCommands()
	}
}

func (cmd HelpCommand) displayHelpPreamble() {
	cmd.UI.DisplayHeader("NAME:")
	cmd.UI.DisplayText(allCommandsIndent+"{{.CommandName}} - {{.CommandDescription}}",
		map[string]interface{}{
			"CommandName":        cmd.Config.BinaryName(),
			"CommandDescription": cmd.UI.TranslateText("A command line tool to interact with Cloud Foundry"),
		})
	cmd.UI.DisplayNewline()

	cmd.UI.DisplayHeader("USAGE:")
	cmd.UI.DisplayText(allCommandsIndent+"{{.CommandName}} {{.CommandUsage}}",
		map[string]interface{}{
			"CommandName":  cmd.Config.BinaryName(),
			"CommandUsage": cmd.UI.TranslateText("[global options] command [arguments...] [command options]"),
		})
	cmd.UI.DisplayNewline()

	cmd.UI.DisplayHeader("VERSION:")
	cmd.UI.DisplayText(allCommandsIndent + cmd.Config.BinaryVersion())
	cmd.UI.DisplayNewline()
}

func (cmd HelpCommand) displayAllCommands() {
	pluginCommands := cmd.getSortedPluginCommands()
	cmdInfo := cmd.Actor.CommandInfos(Commands)
	longestCmd := internal.LongestCommandName(cmdInfo, pluginCommands)

	for _, category := range internal.HelpCategoryList {
		cmd.UI.DisplayHeader(category.CategoryName)

		for _, row := range category.CommandList {
			for _, command := range row {
				cmd.UI.DisplayText(allCommandsIndent+"{{.CommandName}}{{.Gap}}{{.CommandDescription}}",
					map[string]interface{}{
						"CommandName":        cmdInfo[command].Name,
						"CommandDescription": cmd.UI.TranslateText(cmdInfo[command].Description),
						"Gap":                strings.Repeat(" ", longestCmd+1-len(command)),
					})
			}

			cmd.UI.DisplayNewline()
		}
	}

	cmd.UI.DisplayHeader("INSTALLED PLUGIN COMMANDS:")
	for _, pluginCommand := range pluginCommands {
		cmd.UI.DisplayText(allCommandsIndent+"{{.CommandName}}{{.Gap}}{{.CommandDescription}}", map[string]interface{}{
			"CommandName":        pluginCommand.Name,
			"CommandDescription": pluginCommand.HelpText,
			"Gap":                strings.Repeat(" ", longestCmd+1-len(pluginCommand.Name)),
		})
	}
	cmd.UI.DisplayNewline()
}

func (cmd HelpCommand) displayHelpFooter() {
	cmd.UI.DisplayHeader("ENVIRONMENT VARIABLES:")
	cmd.UI.DisplayTable(allCommandsIndent, cmd.environmentalVariablesTableData(), 1)

	cmd.UI.DisplayNewline()

	cmd.UI.DisplayHeader("GLOBAL OPTIONS:")
	cmd.UI.DisplayTable(allCommandsIndent, cmd.globalOptionsTableData(), 25)
}

func (cmd HelpCommand) displayCommonCommands() {
	cmdInfo := cmd.Actor.CommandInfos(Commands)

	cmd.UI.DisplayText("{{.CommandName}} {{.VersionCommand}} {{.Version}}, {{.CLI}}",
		map[string]interface{}{
			"CommandName":    cmd.Config.BinaryName(),
			"VersionCommand": cmd.UI.TranslateText("version"),
			"Version":        cmd.Config.BinaryVersion(),
			"CLI":            cmd.UI.TranslateText("Cloud Foundry command line tool"),
		})
	cmd.UI.DisplayText("{{.Usage}} {{.CommandName}} {{.CommandUsage}}",
		map[string]interface{}{
			"Usage":        cmd.UI.TranslateText("Usage:"),
			"CommandName":  cmd.Config.BinaryName(),
			"CommandUsage": cmd.UI.TranslateText("[global options] command [arguments...] [command options]"),
		})
	cmd.UI.DisplayNewline()

	for _, category := range internal.CommonHelpCategoryList {
		cmd.UI.DisplayHeader(category.CategoryName)
		table := [][]string{}

		for _, row := range category.CommandList {
			finalRow := []string{}

			for _, command := range row {
				separator := ""
				if info, ok := cmdInfo[command]; ok {
					if len(info.Alias) > 0 {
						separator = ","
					}
					finalRow = append(finalRow, fmt.Sprint(info.Name, separator, info.Alias))
				}
			}

			table = append(table, finalRow)
		}

		cmd.UI.DisplayTable(commonCommandsIndent, table, 4)
		cmd.UI.DisplayNewline()
	}

	pluginCommands := cmd.getSortedPluginCommands()

	size := int(math.Ceil(float64(len(pluginCommands)) / 3))
	table := make([][]string, size)
	for i := 0; i < size; i++ {
		table[i] = make([]string, 3)
		for j := 0; j < 3; j++ {
			index := i + j*size
			if index < len(pluginCommands) {
				table[i][j] = pluginCommands[index].Name
			}
		}
	}

	cmd.UI.DisplayHeader("Commands offered by installed plugins:")
	cmd.UI.DisplayTable(commonCommandsIndent, table, 4)
	cmd.UI.DisplayNewline()

	cmd.UI.DisplayHeader("Global options:")
	cmd.UI.DisplayTable(commonCommandsIndent, cmd.globalOptionsTableData(), 25)
	cmd.UI.DisplayNewline()

	cmd.UI.DisplayText("These are commonly used commands. Use 'cf help -a' to see all, with descriptions.")
	cmd.UI.DisplayText("See 'cf help <command>' to read about a specific command.")
}

func (cmd HelpCommand) displayCommand() error {
	cmdInfo, err := cmd.Actor.CommandInfoByName(Commands, cmd.OptionalArgs.CommandName)
	if err != nil {
		if err, ok := err.(sharedaction.ErrorInvalidCommand); ok {
			var found bool
			if cmdInfo, found = cmd.findPlugin(); !found {
				return err
			}
		} else {
			return err
		}
	}

	cmd.UI.DisplayText("NAME:")
	cmd.UI.DisplayText(commandIndent+"{{.CommandName}} - {{.CommandDescription}}",
		map[string]interface{}{
			"CommandName":        cmdInfo.Name,
			"CommandDescription": cmd.UI.TranslateText(cmdInfo.Description),
		})

	cmd.UI.DisplayNewline()
	usageString := strings.Replace(cmdInfo.Usage, "CF_NAME", cmd.Config.BinaryName(), -1)
	cmd.UI.DisplayText("USAGE:")
	cmd.UI.DisplayText(commandIndent+"{{.CommandUsage}}",
		map[string]interface{}{
			"CommandUsage": cmd.UI.TranslateText(usageString),
		})

	if cmdInfo.Alias != "" {
		cmd.UI.DisplayNewline()
		cmd.UI.DisplayText("ALIAS:")
		cmd.UI.DisplayText(commandIndent+"{{.Alias}}",
			map[string]interface{}{
				"Alias": cmdInfo.Alias,
			})
	}

	if len(cmdInfo.Flags) != 0 {
		cmd.UI.DisplayNewline()
		cmd.UI.DisplayText("OPTIONS:")
		nameWidth := internal.LongestFlagWidth(cmdInfo.Flags) + 6
		for _, flag := range cmdInfo.Flags {
			var name string
			if flag.Short != "" && flag.Long != "" {
				name = fmt.Sprintf("--%s, -%s", flag.Long, flag.Short)
			} else if flag.Short != "" {
				name = "-" + flag.Short
			} else {
				name = "--" + flag.Long
			}

			cmd.UI.DisplayText(commandIndent+"{{.Flags}}{{.Spaces}}{{.Description}}",
				map[string]interface{}{
					"Flags":       name,
					"Spaces":      strings.Repeat(" ", nameWidth-len(name)),
					"Description": cmd.UI.TranslateText(flag.Description),
				})
		}
	}

	if len(cmdInfo.Environment) != 0 {
		cmd.UI.DisplayNewline()
		cmd.UI.DisplayText("ENVIRONMENT:")
		for _, envVar := range cmdInfo.Environment {
			cmd.UI.DisplayText(commandIndent+"{{.EnvVar}}{{.Description}}",
				map[string]interface{}{
					"EnvVar":      fmt.Sprintf("%-29s", fmt.Sprintf("%s=%s", envVar.Name, envVar.DefaultValue)),
					"Description": cmd.UI.TranslateText(envVar.Description),
				})
		}
	}

	if len(cmdInfo.RelatedCommands) > 0 {
		cmd.UI.DisplayNewline()
		cmd.UI.DisplayText("SEE ALSO:")
		cmd.UI.DisplayText(commandIndent + strings.Join(cmdInfo.RelatedCommands, ", "))
	}

	return nil
}

func (cmd HelpCommand) environmentalVariablesTableData() [][]string {
	return [][]string{
		{"CF_COLOR=false", cmd.UI.TranslateText("Do not colorize output")},
		{"CF_DIAL_TIMEOUT=5", cmd.UI.TranslateText("Max wait time to establish a connection, including name resolution, in seconds")},
		{"CF_HOME=path/to/dir/", cmd.UI.TranslateText("Override path to default config directory")},
		{"CF_PLUGIN_HOME=path/to/dir/", cmd.UI.TranslateText("Override path to default plugin config directory")},
		{"CF_TRACE=true", cmd.UI.TranslateText("Print API request diagnostics to stdout")},
		{"CF_TRACE=path/to/trace.log", cmd.UI.TranslateText("Append API request diagnostics to a log file")},
		{"https_proxy=proxy.example.com:8080", cmd.UI.TranslateText("Enable HTTP proxying for API requests")},
	}
}

func (cmd HelpCommand) globalOptionsTableData() [][]string {
	return [][]string{
		{"--help, -h", cmd.UI.TranslateText("Show help")},
		{"-v", cmd.UI.TranslateText("Print API request diagnostics to stdout")},
	}
}

func (cmd HelpCommand) findPlugin() (sharedaction.CommandInfo, bool) {
	for _, pluginConfig := range cmd.Config.Plugins() {
		for _, command := range pluginConfig.Commands {
			if command.Name == cmd.OptionalArgs.CommandName {
				return internal.ConvertPluginToCommandInfo(command), true
			}
		}
	}

	return sharedaction.CommandInfo{}, false
}

func (cmd HelpCommand) getSortedPluginCommands() configv3.PluginCommands {
	plugins := cmd.Config.Plugins()

	sortedPluginNames := sorting.Alphabetic{}
	for plugin, _ := range plugins {
		sortedPluginNames = append(sortedPluginNames, plugin)
	}
	sort.Sort(sortedPluginNames)

	pluginCommands := configv3.PluginCommands{}
	for _, pluginName := range sortedPluginNames {
		sortedCommands := plugins[pluginName].Commands
		sort.Sort(sortedCommands)
		pluginCommands = append(pluginCommands, sortedCommands...)
	}

	return pluginCommands
}
