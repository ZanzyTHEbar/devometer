import js from "@eslint/js";
import ts from "@typescript-eslint/eslint-plugin";
import tsParser from "@typescript-eslint/parser";
import solid from "eslint-plugin-solid";

export default [
    js.configs.recommended,
    {
        files: ["src/**/*.{ts,tsx}"],
        languageOptions: {
            parser: tsParser,
            parserOptions: {
                ecmaVersion: "latest",
                sourceType: "module",
                ecmaFeatures: {
                    jsx: true,
                },
            },
            globals: {
                // Browser globals
                window: "readonly",
                document: "readonly",
                fetch: "readonly",
                requestAnimationFrame: "readonly",
                console: "readonly",
                // TypeScript DOM types
                KeyboardEvent: "readonly",
                HTMLInputElement: "readonly",
            },
        },
        plugins: {
            "@typescript-eslint": ts,
            solid,
        },
        rules: {
            ...ts.configs.recommended.rules,
            ...solid.configs.recommended.rules,
            "@typescript-eslint/no-unused-vars": ["error", { argsIgnorePattern: "^_" }],
            "@typescript-eslint/explicit-function-return-type": "off",
            "@typescript-eslint/explicit-module-boundary-types": "off",
            "@typescript-eslint/no-explicit-any": "warn",
            "solid/jsx-no-undef": "error",
            "solid/no-destructure": "error",
            "solid/prefer-for": "error",
            "solid/reactivity": "error",
        },
    },
    {
        files: ["*.config.{js,ts}"],
        rules: {
            "@typescript-eslint/no-var-requires": "off",
        },
    },
];