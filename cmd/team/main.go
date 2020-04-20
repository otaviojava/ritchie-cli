package main

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/ZupIT/ritchie-cli/pkg/prompt"
	"github.com/ZupIT/ritchie-cli/pkg/server"

	"github.com/ZupIT/ritchie-cli/pkg/api"
	"github.com/ZupIT/ritchie-cli/pkg/autocomplete"
	"github.com/ZupIT/ritchie-cli/pkg/cmd"
	"github.com/ZupIT/ritchie-cli/pkg/credential/credteam"
	"github.com/ZupIT/ritchie-cli/pkg/env"
	"github.com/ZupIT/ritchie-cli/pkg/env/envcredential"
	"github.com/ZupIT/ritchie-cli/pkg/formula"
	"github.com/ZupIT/ritchie-cli/pkg/metrics"
	"github.com/ZupIT/ritchie-cli/pkg/rcontext"
	"github.com/ZupIT/ritchie-cli/pkg/security"
	"github.com/ZupIT/ritchie-cli/pkg/security/secteam"
	"github.com/ZupIT/ritchie-cli/pkg/session"
	"github.com/ZupIT/ritchie-cli/pkg/session/sessteam"
	"github.com/ZupIT/ritchie-cli/pkg/workspace"
)

func main() {
	rootCmd := buildCommands()
	if err := rootCmd.Execute(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error: %+v\n", err)
		os.Exit(1)
	}
}

func buildCommands() *cobra.Command {
	userHomeDir := api.UserHomeDir()
	ritchieHomeDir := api.RitchieHomeDir()

	//prompt
	inputText := prompt.NewInputText()
	inputInt := prompt.NewInputInt()
	inputBool := prompt.NewInputBool()
	inputEmail := prompt.NewInputEmail()
	inputPassword := prompt.NewInputPassword()
	inputList := prompt.NewInputList()
	inputURL := prompt.NewInputURL()

	//deps
	sessionManager := session.NewManager(ritchieHomeDir)
	workspaceManager := workspace.NewChecker(ritchieHomeDir)
	srvFinder := server.NewFinder(ritchieHomeDir)
	ctxFinder := rcontext.NewFinder(ritchieHomeDir)
	ctxSetter := rcontext.NewSetter(ritchieHomeDir, ctxFinder)
	ctxRemover := rcontext.NewRemover(ritchieHomeDir, ctxFinder)
	ctxFindSetter := rcontext.NewFindSetter(ritchieHomeDir, ctxFinder, ctxSetter)
	ctxFindRemover := rcontext.NewFindRemover(ritchieHomeDir, ctxFinder, ctxRemover)
	serverSetter := server.NewSetter(ritchieHomeDir)
	repoManager := formula.NewTeamRepoManager(ritchieHomeDir, srvFinder, http.DefaultClient, sessionManager)
	serverValidator := server.NewValidator(srvFinder)
	sessionValidator := sessteam.NewValidator(sessionManager)
	loginManager := secteam.NewLoginManager(
		ritchieHomeDir,
		srvFinder,
		security.OAuthProvider,
		http.DefaultClient,
		sessionManager)
	logoutManager := secteam.NewLogoutManager(security.OAuthProvider, sessionManager, srvFinder)
	userManager := secteam.NewUserManager(srvFinder, http.DefaultClient, sessionManager)
	credSetter := credteam.NewSetter(srvFinder, http.DefaultClient, sessionManager, ctxFinder)
	credFinder := credteam.NewFinder(srvFinder, http.DefaultClient, sessionManager, ctxFinder)
	credSettings := credteam.NewSettings(srvFinder, http.DefaultClient, sessionManager, ctxFinder)
	treeManager := formula.NewTreeManager(ritchieHomeDir, repoManager, api.TeamCoreCmds)
	autocompleteGen := autocomplete.NewGenerator(treeManager)
	credResolver := envcredential.NewResolver(credFinder)
	envResolvers := make(env.Resolvers)
	envResolvers[env.Credential] = credResolver

	formulaRunner := formula.NewRunner(
		ritchieHomeDir,
		envResolvers,
		http.DefaultClient,
		treeManager,
		inputList,
		inputText,
		inputBool)
	formulaCreator := formula.NewCreator(userHomeDir, treeManager)

	//commands
	rootCmd := cmd.NewRootCmd(
		workspaceManager,
		loginManager,
		repoManager,
		serverValidator,
		sessionValidator,
		api.Team,
		inputText,
		inputPassword)

	// level 1
	autocompleteCmd := cmd.NewAutocompleteCmd()
	addCmd := cmd.NewAddCmd()
	cleanCmd := cmd.NewCleanCmd()
	createCmd := cmd.NewCreateCmd()
	deleteCmd := cmd.NewDeleteCmd()
	listCmd := cmd.NewListCmd()
	loginCmd := cmd.NewLoginCmd(loginManager, repoManager, inputText)
	logoutCmd := cmd.NewLogoutCmd(logoutManager)
	setCmd := cmd.NewSetCmd()
	showCmd := cmd.NewShowCmd()
	updateCmd := cmd.NewUpdateCmd()

	// level 2
	setCredentialCmd := cmd.NewTeamSetCredentialCmd(
		credSetter,
		credSettings,
		inputText,
		inputBool,
		inputList,
		inputPassword)
	createUserCmd := cmd.NewCreateUserCmd(userManager, inputText, inputEmail, inputPassword)
	deleteUserCmd := cmd.NewDeleteUserCmd(userManager, inputBool, inputText)
	deleteCtxCmd := cmd.NewDeleteContextCmd(ctxFindRemover, inputBool, inputList)
	setCtxCmd := cmd.NewSetContextCmd(ctxFindSetter, inputText, inputList)
	setServerCmd := cmd.NewSetServerCmd(serverSetter, inputURL)
	showCtxCmd := cmd.NewShowContextCmd(ctxFinder)
	addRepoCmd := cmd.NewAddRepoCmd(repoManager, inputText, inputURL, inputInt)
	cleanRepoCmd := cmd.NewCleanRepoCmd(repoManager, inputText)
	deleteRepoCmd := cmd.NewDeleteRepoCmd(repoManager, inputText)
	listRepoCmd := cmd.NewListRepoCmd(repoManager)
	updateRepoCmd := cmd.NewUpdateRepoCmd(repoManager)
	autocompleteZsh := cmd.NewAutocompleteZsh(autocompleteGen)
	autocompleteBash := cmd.NewAutocompleteBash(autocompleteGen)
	createFormulaCmd := cmd.NewCreateFormulaCmd(formulaCreator, inputText)

	autocompleteCmd.AddCommand(autocompleteZsh, autocompleteBash)
	addCmd.AddCommand(addRepoCmd)
	cleanCmd.AddCommand(cleanRepoCmd)
	createCmd.AddCommand(createUserCmd, createFormulaCmd)
	deleteCmd.AddCommand(deleteUserCmd, deleteRepoCmd, deleteCtxCmd)
	listCmd.AddCommand(listRepoCmd)
	setCmd.AddCommand(setCredentialCmd, setCtxCmd, setServerCmd)
	showCmd.AddCommand(showCtxCmd)
	updateCmd.AddCommand(updateRepoCmd)

	rootCmd.AddCommand(
		addCmd,
		autocompleteCmd,
		cleanCmd,
		createCmd,
		deleteCmd,
		listCmd,
		loginCmd,
		logoutCmd,
		setCmd,
		showCmd,
		updateCmd)

	formulaCmd := cmd.NewFormulaCommand(api.TeamCoreCmds, treeManager, formulaRunner)
	if err := formulaCmd.Add(rootCmd); err != nil {
		panic(err)
	}

	sendMetrics(sessionManager)

	return rootCmd
}

func sendMetrics(sm session.DefaultManager) {
	srvFinder := server.NewFinder(api.RitchieHomeDir())
	metricsManager := metrics.NewSender(srvFinder, &http.Client{Timeout: 2 * time.Second}, sm)
	go metricsManager.SendCommand()
}
