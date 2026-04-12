- VULNERABILITIES:
  - **Temporal Rigidity**: The answer "Paris" is correct now, but capitals can and do change (e.g., Brazil, Germany, Kazakhstan). A system optimized only for "conciseness" might fail to provide a historically accurate answer if the user's context implies a different time period (e.g., "What was the capital of France in 1942?"). The answer, while technically Paris, would be misleading without mentioning Vichy.
  - **Unverified Confidence**: The "Confidence Signaling" is based on the prevalence of data in the training set, not on a real-time verification of facts. If the model were trained on a dataset where "Lyon is the capital of France" was a dominant statement, it would be confidently and concisely wrong. This represents a vulnerability to data poisoning or outdated information.
  - **Semantic Oversimplification**: The term "capital" can be ambiguous (e.g., administrative, legislative, judicial, economic). For France, Paris is the answer to all, but for a country like South Africa or Bolivia, a single-city answer is incorrect. The optimization for "Clarity" might falsely simplify a multi-faceted reality.

- EDGE_CASES:
  - **Countries with Multiple Capitals**: A query about South Africa (Pretoria, Cape Town, Bloemfontein) would fail if the system is hard-coded to provide a single, concise answer.
  - **De Facto vs. De Jure Capitals**: The Netherlands has Amsterdam as its de jure capital, but The Hague is the de facto seat of government. A simple answer could be incomplete.
  - **Future/Hypothetical Scenarios**: Queries like "What will be the capital of France in 2050?" or "What is the capital of the European Union?" (which has no official capital but seats of power) would challenge the static, fact-based response model.

- PATCHES:
  - **Contextual Timestamping**: For facts that can change over time, the model should state them with temporal context, e.g., "As of [current date], the capital of France is Paris."
  - **Disambiguation for Ambiguous Queries**: When a query has multiple valid interpretations (like "capital"), the model should provide a more comprehensive answer. E.g., "The administrative, legislative, and economic capital of France is Paris. In some countries, these functions are split between different cities."
  - **Source-Checking/Confidence Nuance**: Instead of absolute confidence, the model could signal confidence based on the consensus of multiple reliable sources, and even cite them. This moves from "I am confident" to "There is a strong consensus that...".

- RISK_LEVEL: 2/10
  - For this specific, well-known fact, the risk is negligible. However, the underlying logic (prioritizing conciseness and unverified confidence) is a systemic weakness. Applying this same optimization strategy to more complex, evolving, or contentious topics poses a moderate risk of confidently providing incorrect or misleading information.
