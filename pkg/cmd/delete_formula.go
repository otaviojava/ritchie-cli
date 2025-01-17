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
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ZupIT/ritchie-cli/pkg/formula"
	"github.com/ZupIT/ritchie-cli/pkg/formula/repo/repoutil"
	"github.com/ZupIT/ritchie-cli/pkg/prompt"
	"github.com/ZupIT/ritchie-cli/pkg/stdin"
	"github.com/ZupIT/ritchie-cli/pkg/stream"
)

const (
	msgFormulaNotFound         = "could not find formula"
	msgIncorrectFormulaName    = "formula name is incorrect"
	foundFormulaQuestion       = "we found a formula, which one do you want to delete: "
	docsDir                    = "docs"
	srcDir                     = "src"
	questionSelectFormulaGroup = "Select a formula or group: "
	optionOtherFormula         = "Another formula"
	workspaceFlagName          = "workspace"
	formulaFlagName            = "formula"
)

var (
	ErrCouldNotFindFormula  = errors.New(msgFormulaNotFound)
	ErrIncorrectFormulaName = errors.New(msgIncorrectFormulaName)
)

type (
	deleteFormulaStdin struct {
		WorkspacePath string `json:"workspace_path"`
		Formula       string `json:"formula"`
	}

	deleteFormula struct {
		Workspace formula.Workspace
		Formula   []string
	}

	deleteFormulaCmd struct {
		userHomeDir     string
		ritchieHomeDir  string
		workspace       formula.WorkspaceAddLister
		directory       stream.DirListChecker
		inBool          prompt.InputBool
		inTextValidator prompt.InputTextValidator
		inList          prompt.InputList
		inPath          prompt.InputPath
		treeGen         formula.TreeGenerator
		fileManager     stream.FileWriteRemover
	}
)

var deleteFormulas = flags{
	{
		name:        workspaceFlagName,
		kind:        reflect.String,
		defValue:    "",
		description: "workspace name (e.g.: Default or default)",
	},
	{
		name:        formulaFlagName,
		kind:        reflect.String,
		defValue:    "",
		description: "formula to remove (e.g.: rit test delete)",
	},
}

func NewDeleteFormulaCmd(
	userHomeDir string,
	ritchieHomeDir string,
	workspace formula.WorkspaceAddLister,
	directory stream.DirListChecker,
	inBool prompt.InputBool,
	inTextValidator prompt.InputTextValidator,
	inList prompt.InputList,
	inPath prompt.InputPath,
	treeGen formula.TreeGenerator,
	fileManager stream.FileWriteRemover,
) *cobra.Command {
	d := deleteFormulaCmd{
		userHomeDir,
		ritchieHomeDir,
		workspace,
		directory,
		inBool,
		inTextValidator,
		inList,
		inPath,
		treeGen,
		fileManager,
	}

	cmd := &cobra.Command{
		Use:       "formula",
		Short:     "Delete specific formula",
		Example:   "rit delete formula",
		RunE:      RunFuncE(d.runStdin(), d.runCmd()),
		ValidArgs: []string{""},
		Args:      cobra.OnlyValidArgs,
	}

	addReservedFlags(cmd.Flags(), deleteFormulas)

	return cmd
}

func (d deleteFormulaCmd) runCmd() CommandRunnerFunc {
	return func(cmd *cobra.Command, args []string) error {
		deleteFormula, err := d.resolveInput(cmd)
		if err != nil {
			return err
		}

		wspaceName := repoutil.LocalName(deleteFormula.Workspace.Name)
		wspacePath := deleteFormula.Workspace.Dir
		groups := deleteFormula.Formula

		if len(groups) == 0 {
			return nil
		}

		// Delete formula on user workspace
		if err := d.deleteFormula(wspacePath, groups, 0); err != nil {
			return err
		}

		ritchieLocalWorkspace := filepath.Join(d.ritchieHomeDir, "repos", wspaceName.String())
		if d.formulaExistsInWorkspace(ritchieLocalWorkspace, groups) {
			if err := d.deleteFormula(ritchieLocalWorkspace, groups, 0); err != nil {
				return err
			}
			if err := d.recreateTreeJSON(ritchieLocalWorkspace); err != nil {
				return err
			}
		}

		prompt.Success("Formula successfully deleted!")

		return nil
	}
}

func (d deleteFormulaCmd) runStdin() CommandRunnerFunc {
	return func(cmd *cobra.Command, args []string) error {
		deleteStdin := deleteFormulaStdin{}

		if err := stdin.ReadJson(cmd.InOrStdin(), &deleteStdin); err != nil {
			return err
		}

		// rit my amazing formula -> ['my', 'amazing', 'formula']
		groups, err := getGroupsFromFormulaName(deleteStdin.Formula)
		if err != nil {
			return err
		}

		// Delete formula on user workspace
		if err := d.deleteFormula(deleteStdin.WorkspacePath, groups, 0); err != nil {
			return err
		}

		workspaces, err := d.workspace.List()
		if err != nil {
			return err
		}

		var wspace string
		for workspaceName, path := range workspaces {
			if strings.EqualFold(path, deleteStdin.WorkspacePath) {
				wspace = workspaceName
			}
		}

		wspaceName := repoutil.LocalName(wspace)
		ritchieLocalWorkspace := filepath.Join(d.ritchieHomeDir, "repos", wspaceName.String())
		if d.formulaExistsInWorkspace(ritchieLocalWorkspace, groups) {
			if err := d.deleteFormula(ritchieLocalWorkspace, groups, 0); err != nil {
				return err
			}
			if err := d.recreateTreeJSON(ritchieLocalWorkspace); err != nil {
				return err
			}
		}

		return nil
	}
}

func (d *deleteFormulaCmd) resolveInput(cmd *cobra.Command) (deleteFormula, error) {
	if IsFlagInput(cmd) {
		return d.resolveFlags(cmd)
	}

	return d.resolvePrompt()
}

func (d *deleteFormulaCmd) resolvePrompt() (deleteFormula, error) {
	workspaces, err := d.workspace.List()
	if err != nil {
		return deleteFormula{}, err
	}

	wspace, err := FormulaWorkspaceInput(workspaces, d.inList, d.inTextValidator, d.inPath)
	if err != nil {
		return deleteFormula{}, err
	}

	if err := d.workspace.Add(wspace); err != nil {
		return deleteFormula{}, err
	}

	groups, err := d.readFormulas(wspace.Dir, "rit")
	if err != nil {
		return deleteFormula{}, err
	}

	if groups == nil {
		return deleteFormula{}, ErrCouldNotFindFormula
	}

	question := fmt.Sprintf("Are you sure you want to delete the formula: rit %s", strings.Join(groups, " "))
	ans, err := d.inBool.Bool(question, []string{"no", "yes"})
	if err != nil {
		return deleteFormula{}, err
	}
	if !ans {
		return deleteFormula{}, nil
	}

	return deleteFormula{Workspace: wspace, Formula: groups}, nil
}

func (d *deleteFormulaCmd) resolveFlags(cmd *cobra.Command) (deleteFormula, error) {
	workspace, err := cmd.Flags().GetString(workspaceFlagName)
	if err != nil {
		return deleteFormula{}, err
	} else if workspace == "" {
		return deleteFormula{}, errors.New(missingFlagText(workspaceFlagName))
	}

	formulaGroup, err := cmd.Flags().GetString(formulaFlagName)
	if err != nil {
		return deleteFormula{}, err
	} else if formulaGroup == "" {
		return deleteFormula{}, errors.New(missingFlagText(formulaFlagName))
	}

	workspaces, err := d.workspace.List()
	if err != nil {
		return deleteFormula{}, err
	}

	var wspace formula.Workspace
	for workspaceName, path := range workspaces {
		if strings.EqualFold(workspaceName, workspace) {
			wspace = formula.Workspace{Name: workspaceName, Dir: path}
		}
	}

	if wspace.Name == "" {
		return deleteFormula{}, errors.New("no workspace found with this name")
	}

	groups, err := getGroupsFromFormulaName(formulaGroup)
	if err != nil {
		return deleteFormula{}, err
	}

	return deleteFormula{wspace, groups}, nil
}

func (d deleteFormulaCmd) readFormulas(dir string, currentFormula string) ([]string, error) {
	dirs, err := d.directory.List(dir, false)
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

		response, err = d.inList.List(foundFormulaQuestion, formulaOptions)
		if err != nil {
			return nil, err
		}
		if response == currentFormula {
			return groups, nil
		}
		dirs = removeFromArray(dirs, srcDir)
	}

	selected, err := d.inList.List(questionSelectFormulaGroup, dirs)
	if err != nil {
		return nil, err
	}

	newFormulaSelected := fmt.Sprintf("%s %s", currentFormula, selected)

	var aux []string
	aux, err = d.readFormulas(filepath.Join(dir, selected), newFormulaSelected)
	if err != nil {
		return nil, err
	}

	aux = append([]string{selected}, aux...)
	groups = append(groups, aux...)

	return groups, nil
}

func (d deleteFormulaCmd) deleteFormula(path string, groups []string, index int) error {
	if index == len(groups) {
		nested, err := nestedFormula(path)
		if err != nil {
			return err
		}

		if nested {
			return d.safeRemoveFormula(path)
		}

		return os.RemoveAll(path)
	}

	newPath := filepath.Join(path, groups[index])
	if err := d.deleteFormula(newPath, groups, index+1); err != nil {
		return err
	}

	if index == 0 {
		return nil
	}

	ok, err := canDelete(path)
	if err != nil {
		return err
	}

	if ok {
		if err := os.RemoveAll(path); err != nil {
			return err
		}
	}

	return nil
}

func (d deleteFormulaCmd) safeRemoveFormula(path string) error {
	files, err := ioutil.ReadDir(path)
	if err != nil {
		return err
	}

	for _, file := range files {
		if file.IsDir() && (file.Name() == "src" || file.Name() == "bin") {
			pathToDelete := filepath.Join(path, file.Name())
			if err := os.RemoveAll(pathToDelete); err != nil {
				return err
			}
		} else if !file.IsDir() {
			pathToDelete := filepath.Join(path, file.Name())
			if err := d.fileManager.Remove(pathToDelete); err != nil {
				return err
			}
		}
	}

	return nil
}

func (d deleteFormulaCmd) recreateTreeJSON(ritchieLocalWorkspace string) error {
	localTree, err := d.treeGen.Generate(ritchieLocalWorkspace)
	if err != nil {
		return err
	}

	jsonString, _ := json.MarshalIndent(localTree, "", "\t")
	pathLocalTreeJSON := filepath.Join(ritchieLocalWorkspace, "tree.json")
	if err = d.fileManager.Write(pathLocalTreeJSON, jsonString); err != nil {
		return err
	}

	return nil
}

func (d deleteFormulaCmd) formulaExistsInWorkspace(path string, groups []string) bool {
	for _, group := range groups {
		path = filepath.Join(path, group)
	}

	return d.directory.Exists(path)
}

func nestedFormula(path string) (bool, error) {
	files, err := ioutil.ReadDir(path)
	if err != nil {
		return false, err
	}

	for _, file := range files {
		if file.IsDir() && file.Name() != "src" && file.Name() != "bin" {
			return true, nil
		}
	}

	return false, nil
}

func canDelete(path string) (bool, error) {
	files, err := ioutil.ReadDir(path)
	if err != nil {
		return false, err
	}

	for _, file := range files {
		if file.IsDir() {
			return false, nil
		}
	}

	return true, nil
}

func getGroupsFromFormulaName(formulaName string) ([]string, error) {
	groups := strings.Split(formulaName, " ")

	if len(groups) > 0 && groups[0] == cmdUse {
		return groups[1:], nil
	}

	return nil, ErrIncorrectFormulaName
}

func isFormula(dirs []string) bool {
	for _, dir := range dirs {
		if dir == srcDir {
			return true
		}
	}

	return false
}

func hasFormulaInDir(dirs []string) bool {
	dirs = removeFromArray(dirs, docsDir)
	dirs = removeFromArray(dirs, srcDir)

	return len(dirs) > 0
}

func removeFromArray(ss []string, r string) []string {
	for i, s := range ss {
		if s == r {
			return append(ss[:i], ss[i+1:]...)
		}
	}

	return ss
}
