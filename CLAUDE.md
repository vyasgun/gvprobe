# CLAUDE.md

Primary guidance for Claude Code when working on **gvprobe**.

## Project Goal

gvprobe is a CLI tool for sending real DHCP Discover and ARP request packets to gvproxy via vfkit unixgram socket.  
Long-term goal: evolve it into an intelligent test harness with agentic capabilities.

## How You Should Work

- Prioritize clarity, consistency, and readability in both code and output.
- Default output must be clean and human-first. `--machine` is only for raw gopacket decode.
- Keep all `trace` commands consistent in flags and output style.
- Respect gvproxy’s anti-spoofing protection — document limitations clearly.
- Prefer gopacket for packet construction.

## Style Rules

- When adding features: First propose the **desired user output**, then the code.
- For `--count` mode: Use compact numbered lines + a final summary.
- Be practical and concise.
- Ask clarifying questions if needed.

See `README.md` for usage and `ARCHITECTURE.md` for layout and flows. If you keep a project skill, put it under `.claude/skills/` and link it here.