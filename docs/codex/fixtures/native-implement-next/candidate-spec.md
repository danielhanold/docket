# Trim surrounding whitespace in greeting

## Purpose and scope

Make Greet normalize surrounding Unicode whitespace while preserving its existing greeting format. This is the single real implementation task in the disposable native-dispatch POC. It is fixture change 1; it does not implement production Docket change 423.

## Required behavior

Apply strings.TrimSpace to the supplied name before the existing empty-name decision. Return "Hello!" for an empty result, otherwise return "Hello, " plus the trimmed name plus "!". Preserve interior spaces and punctuation. Do not add external dependencies or change the exported function signature.

Required examples:
- "Ada" -> "Hello, Ada!"
- "" -> "Hello!"
- "  Ada  " -> "Hello, Ada!"
- "\tAda\n" -> "Hello, Ada!"
- surrounding Unicode nonbreaking spaces around "Ada" -> "Hello, Ada!"
- whitespace-only input, including Unicode whitespace -> "Hello!"
- "  Ada  Lovelace  " -> "Hello, Ada  Lovelace!"
