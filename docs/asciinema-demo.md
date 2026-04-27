# Asciinema Demo

Suggested recording flow for a short `skill-organizer` demo.

## Goal

Show how a skill added directly to `~/.agents/skills` is detected as unmanaged, then moved into the organized source tree and exposed again as a generated flattened target entry.

## Demo Script

```bash
cd ~/.agents

ls
ls skills
ls skills-organized

npx skills add https://github.com/terrylica/cc-skills --skill asciinema-recorder

skill-organizer status --config ~/.agents/.skill-organizer.yml

skill-organizer skill move-unmanaged --config ~/.agents/.skill-organizer.yml
```

Move the skill to:

```text
3rdparty/asciinema/asciinema-recorder
```

Then continue with:

```bash
skill-organizer status --config ~/.agents/.skill-organizer.yml

ls skills-organized/thirdparty/asciinema
ls skills | grep asciinema
ls skills

skill-organizer skill disable 3rdparty/asciinema/asciinema-recorder --config ~/.agents/.skill-organizer.yml
skill-organizer status --config ~/.agents/.skill-organizer.yml

skill-organizer skill enable 3rdparty/asciinema/asciinema-recorder --config ~/.agents/.skill-organizer.yml
skill-organizer status --config ~/.agents/.skill-organizer.yml
```

## What To Highlight

- The target folder is a generated compatibility layer for tools.
- The real source of truth is `skills-organized/`.
- `move-unmanaged` brings manual or third-party skills under organized management.
- Disabling a skill hides it from the target without deleting its real files.
- The generated folder still contains `IMPORTANT.md` to signal that it should not be edited directly.
