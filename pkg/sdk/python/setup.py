"""
Setup script for SuperAgent Verifier Python SDK.
"""

from setuptools import setup, find_packages

with open("README.md", "r", encoding="utf-8") as fh:
    long_description = fh.read()

setup(
    name="superagent-verifier",
    version="1.0.0",
    author="SuperAgent Team",
    author_email="team@superagent.io",
    description="Python SDK for SuperAgent LLMsVerifier API",
    long_description=long_description,
    long_description_content_type="text/markdown",
    url="https://github.com/superagent/superagent",
    packages=find_packages(),
    classifiers=[
        "Development Status :: 5 - Production/Stable",
        "Intended Audience :: Developers",
        "License :: OSI Approved :: MIT License",
        "Operating System :: OS Independent",
        "Programming Language :: Python :: 3",
        "Programming Language :: Python :: 3.8",
        "Programming Language :: Python :: 3.9",
        "Programming Language :: Python :: 3.10",
        "Programming Language :: Python :: 3.11",
        "Programming Language :: Python :: 3.12",
        "Topic :: Software Development :: Libraries :: Python Modules",
        "Topic :: Scientific/Engineering :: Artificial Intelligence",
    ],
    python_requires=">=3.8",
    install_requires=[
        "requests>=2.25.0",
    ],
    extras_require={
        "dev": [
            "pytest>=7.0.0",
            "pytest-cov>=3.0.0",
            "pytest-asyncio>=0.20.0",
            "responses>=0.22.0",
            "black>=22.0.0",
            "mypy>=0.990",
            "types-requests>=2.28.0",
        ],
    },
    keywords="llm verification scoring health monitoring superagent",
    project_urls={
        "Bug Tracker": "https://github.com/superagent/superagent/issues",
        "Documentation": "https://superagent.io/docs/sdk/python",
        "Source Code": "https://github.com/superagent/superagent/tree/main/pkg/sdk/python",
    },
)
