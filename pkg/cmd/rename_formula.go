/*
 * Copyright 2020 ZUP IT SERVICOS EM TECNOLOGIA E INOVACAO SA
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package cmd

import (
	"fmt"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ZupIT/ritchie-cli/pkg/formula"
	work "github.com/ZupIT/ritchie-cli/pkg/formula/workspace"
	"github.com/ZupIT/ritchie-cli/pkg/prompt"
	"github.com/ZupIT/ritchie-cli/pkg/stream"
)

const (
	workspaceFlagName           = "workspace"
	workspaceFlagDescription    = "name of workspace to rename"
	oldFormulaFlagName          = "old-name-formula"
	oldFormulaFlagDescription   = "old name of formula to rename"
	newFormulaFlagName          = "new-name-formula"
	newFormulaFlagDescription   = "new name of formula to rename"
	foundFormulaRenamedQuestion = "we found a formula, which one do you want to rename: "

	ErrFormula = "This formula '%s' dont's exists on this workspace = '%s'"
)

var renameWorkspaceFlags = flags{
	{
		name:        workspaceFlagName,
		kind:        reflect.String,
		defValue:    "",
		description: workspaceFlagDescription,
	},
	{
		name:        oldFormulaFlagName,
		kind:        reflect.String,
		defValue:    "",
		description: oldFormulaFlagDescription,
	},
	{
		name:        newFormulaFlagName,
		kind:        reflect.String,
		defValue:    "",
		description: newFormulaFlagDescription,
	},
}

type resultRenameInput struct {
	workspace              formula.Workspace
	oldFormula, newFormula string
	err                    error
}

// renameFormulaCmd type for add formula command.
type renameFormulaCmd struct {
	workspace       formula.WorkspaceAddListHasher
	inText          prompt.InputText
	inList          prompt.InputList
	inPath          prompt.InputPath
	inTextValidator prompt.InputTextValidator
	directory       stream.DirListChecker
	userHomeDir     string
}

// New renameFormulaCmd rename a cmd instance.
func NewRenameFormulaCmd(
	workspace formula.WorkspaceAddListHasher,
	inText prompt.InputText,
	inList prompt.InputList,
	inPath prompt.InputPath,
	inTextValidator prompt.InputTextValidator,
	directory stream.DirListChecker,
	userHomeDir string,
) *cobra.Command {
	r := renameFormulaCmd{
		workspace:       workspace,
		inText:          inText,
		inList:          inList,
		inPath:          inPath,
		inTextValidator: inTextValidator,
		directory:       directory,
		userHomeDir:     userHomeDir,
	}

	cmd := &cobra.Command{
		Use:       "formula",
		Short:     "Rename a formula",
		Example:   "rit rename formula",
		RunE:      r.runFormula(),
		ValidArgs: []string{""},
		Args:      cobra.OnlyValidArgs,
	}

	addReservedFlags(cmd.Flags(), renameWorkspaceFlags)

	return cmd
}

func (r *renameFormulaCmd) runFormula() CommandRunnerFunc {
	return func(cmd *cobra.Command, args []string) error {
		result := r.resolveInput(cmd)
		if result.err != nil {
			return result.err
		}

		fmt.Println(result.workspace, result.newFormula, result.oldFormula)

		return nil
	}
}

func (r *renameFormulaCmd) resolveInput(cmd *cobra.Command) resultRenameInput {
	var result resultRenameInput
	workspaces, err := r.workspace.List()
	if err != nil {
		result.err = err
		return result
	}
	if IsFlagInput(cmd) {
		return r.resolveFlags(cmd, workspaces)
	}
	return r.resolvePrompt(workspaces)
}

func (r *renameFormulaCmd) resolveFlags(cmd *cobra.Command, workspaces formula.Workspaces) resultRenameInput {
	// Default (/home/bruna/ritchie-formulas-local)
	// rit test sandokan
	var result resultRenameInput
	flagError := "please provide a value for '%s'"

	workspaceName, err := cmd.Flags().GetString(workspaceFlagName)
	if err != nil {
		result.err = err
		return result
	} else if workspaceName == "" {
		result.err = fmt.Errorf(flagError, workspaceFlagName)
		return result
	}
	workspaces[formula.DefaultWorkspaceName] = filepath.Join(r.userHomeDir, formula.DefaultWorkspaceDir)
	dir, exists := workspaces[workspaceName]
	if !exists {
		result.err = work.ErrInvalidWorkspace
		return result
	}
	result.workspace.Dir = dir
	result.workspace.Name = workspaceName

	oldFormula, err := cmd.Flags().GetString(oldFormulaFlagName)
	if err != nil {
		result.err = err
		return result
	} else if oldFormula == "" {
		result.err = fmt.Errorf(flagError, oldFormulaFlagName)
		return result
	}
	if !r.formulaExistsInWorkspace(result.workspace.Dir, oldFormula) {
		result.err = fmt.Errorf(ErrFormula, oldFormula, result.workspace.Dir)
		return result
	}
	result.oldFormula = oldFormula

	newFormula, err := cmd.Flags().GetString(newFormulaFlagName)
	if err != nil {
		result.err = err
		return result
	} else if newFormula == "" {
		result.err = fmt.Errorf(flagError, newFormulaFlagName)
		return result
	}
	if !r.formulaExistsInWorkspace(result.workspace.Dir, newFormula) {
		result.err = fmt.Errorf(ErrFormula, newFormula, result.workspace.Dir)
		return result
	}
	result.newFormula = newFormula

	return result
}

func (r *renameFormulaCmd) resolvePrompt(workspaces formula.Workspaces) resultRenameInput {
	var result resultRenameInput

	wspace, err := FormulaWorkspaceInput(workspaces, r.inList, r.inText, r.inPath)
	if err != nil {
		result.err = err
		return result
	}
	result.workspace = wspace

	oldFormula, err := r.readFormulas(wspace.Dir, "rit")
	if err != nil {
		result.err = err
		return result
	}
	if oldFormula == nil {
		result.err = ErrCouldNotFindFormula
		return result
	}
	result.oldFormula = strings.Join(oldFormula, " ")

	newFormula, err := r.inTextValidator.Text(formulaCmdLabel, r.surveyCmdValidator, formulaCmdHelper)
	if err != nil {
		result.err = err
		return result
	}
	result.newFormula = newFormula

	return result
}

func (r *renameFormulaCmd) readFormulas(dir string, currentFormula string) ([]string, error) {
	dirs, err := r.directory.List(dir, false)
	if err != nil {
		return nil, err
	}

	dirs = removeFromArray(dirs, docsDir)

	var groups []string
	var formulaOptions []string
	var response string

	if isFormula(dirs) {
		if !hasFormulaInDir(dirs) {
			return groups, nil
		}

		formulaOptions = append(formulaOptions, currentFormula, optionOtherFormula)

		response, err = r.inList.List(foundFormulaRenamedQuestion, formulaOptions)
		if err != nil {
			return nil, err
		}
		if response == currentFormula {
			return groups, nil
		}
		dirs = removeFromArray(dirs, srcDir)
	}

	selected, err := r.inList.List(questionSelectFormulaGroup, dirs)
	if err != nil {
		return nil, err
	}

	newFormulaSelected := fmt.Sprintf("%s %s", currentFormula, selected)

	var aux []string
	aux, err = r.readFormulas(filepath.Join(dir, selected), newFormulaSelected)
	if err != nil {
		return nil, err
	}

	aux = append([]string{selected}, aux...)
	groups = append(groups, aux...)

	return groups, nil
}

func (r *renameFormulaCmd) formulaExistsInWorkspace(path string, formula string) bool {
	fc := cleanFormula(formula)
	for _, group := range fc {
		path = filepath.Join(path, group)
	}

	return r.directory.Exists(path)
}

func cleanFormula(formula string) []string {
	formulaSplited := strings.Split(formula, " ")
	return formulaSplited[1:]
}

func (r *renameFormulaCmd) surveyCmdValidator(cmd interface{}) error {
	if err := formulaCommandValidator(cmd.(string)); err != nil {
		return err
	}

	return nil
}
