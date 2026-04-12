- VULNERABILITIES: [
    "**Unstated Assumptions**: Assumes the user is asking about the present-day, political capital of metropolitan France, ignoring historical context (e.g., Vichy France) and administrative complexity (overseas territories).",
    "**Lack of Verification**: The statement is presented as absolute fact without any sourcing or citation, making it vulnerable to data poisoning attacks on its training set and impossible for a user to verify.",
    "**Semantic Rigidity**: Fails to recognize the potential ambiguity in the word 'capital' (e.g., cultural, economic, gastronomic capital), leading to potentially unhelpful answers for more nuanced queries."
]
- EDGE_CASES: [
    "**Historical Queries**: A user asking 'What was the capital of France in 1942?' might receive the incorrect answer 'Paris' instead of 'Vichy'.",
    "**Geographically Nuanced Queries**: A user studying French overseas territories might find the answer incomplete as it omits regional capitals.",
    "**Figurative/Domain-Specific Queries**: A user asking 'What is the wine capital of France?' would be misunderstood if the model only provides 'Paris'."
]
- PATCHES: [
    "**Temporal and Contextual Scoping**: Prepend the answer with a qualifier like 'Currently, the political capital of France is...' to address temporal and semantic ambiguity.",
    "**Layered Information**: Provide the direct answer first, then offer a brief, optional expansion covering common nuances. Example: 'Paris is the current capital of France. Historically, other cities like Vichy have served as the seat of government.'",
    "**Confidence & Sourcing (Ideal)**: In a more advanced system, link to a high-authority source to back up the claim, transforming the answer from an unverified assertion to a sourced fact."
]
- RISK_LEVEL: [0.1]
