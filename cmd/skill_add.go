package cmd

import (
	"fmt"
	"time"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"

	"github.com/sergiocarracedo/skill-organizer/cli/internal/library"
	"github.com/sergiocarracedo/skill-organizer/cli/internal/remote"
	syncpkg "github.com/sergiocarracedo/skill-organizer/cli/internal/sync"
)

func newSkillAddCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "add <ref>",
		Short: "Install a remote skill into the organized source tree",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			configFile, location, err := loadResolvedLocation()
			if err != nil {
				return err
			}

			location, err = promptInstallScope(location)
			if err != nil {
				return err
			}

			service, err := newRemoteService()
			if err != nil {
				return err
			}

			provider, resolved, err := service.Resolve(args[0])
			if err != nil {
				return err
			}

			selected, err := selectRemoteSkills(resolved)
			if err != nil {
				return err
			}
			if len(selected) == 0 {
				pterm.Info.Println("No skills selected")
				return nil
			}

			destinationParent, err := promptInstallDestination(location)
			if err != nil {
				return err
			}

			bundles := make([]remote.SkillBundle, 0, len(selected))
			destinations := make(map[string]string, len(selected))
			confirmed := make([]remote.SkillSummary, 0, len(selected))
			for _, skill := range selected {
				audit, auditErr := service.Audit(provider, skill)
				if auditErr == nil {
					renderAuditReport(audit)
				} else {
					pterm.Warning.Printfln("Audit unavailable for %s: %v", skill.Name, auditErr)
				}

				ok, err := confirm(fmt.Sprintf("Install %s from %s?", skill.Name, skill.SourceURL), true)
				if err != nil {
					return err
				}
				if !ok {
					continue
				}
				confirmed = append(confirmed, skill)
			}

			for _, skill := range confirmed {
				bundle, err := service.FetchSkill(provider, skill)
				if err != nil {
					return err
				}

				bundles = append(bundles, bundle)
				destinations[skill.ID] = destinationParent
			}

			if len(bundles) == 0 {
				pterm.Info.Println("No skills selected for installation")
				return nil
			}

			installed, err := library.Install(library.InstallRequest{
				Location:         location,
				DestinationPaths: destinations,
				Bundles:          bundles,
			})
			if err != nil {
				return err
			}

			if err := library.GarbageCollectBackups(location.Source, 20, time.Now().UTC()); err != nil {
				return err
			}

			result, err := syncpkg.Run(location)
			if err != nil {
				return err
			}

			for _, entry := range installed {
				pterm.Success.Printfln("Installed %s at %s", entry.Summary.Name, entry.SourceSkill.RelativePath)
			}
			printSyncResult(configFile, result)
			return nil
		},
	}
}
