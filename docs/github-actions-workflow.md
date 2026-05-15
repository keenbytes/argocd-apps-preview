# GitHub Actions Workflow

See a snippet with steps that generate a diff and post it to a PR.

````yaml
# Install prerequisites etc.
      - name: Dump applications from main
        env:
          REPO_NAME: ${{github.repository}}
        run: |
          app-of-apps-dump --manifests ./manifests --output-apps ./outputs-main/ \
            --replace-repo-url https://github.com/${{env.REPO_NAME}} \
            --replace-target-revision main

      - name: Dump applications from branch
        env:
          REPO_NAME: ${{github.repository}}
          BRANCH_NAME: ${{github.head_ref}}
        run: |
          app-of-apps-dump --manifests ./manifests --output-apps ./outputs-${{env.BRANCH_NAME}}/ \
            --replace-repo-url https://github.com/${{env.REPO_NAME}} \
            --replace-target-revision ${{env.BRANCH_NAME}}

      - name: Generate diff
        env:
          REPO_NAME: ${{github.repository}}
          BRANCH_NAME: ${{github.head_ref}}
        run: |
          app-of-apps-diff \
            --apps-base ./outputs-main/ \
            --apps-target ./outputs-${{env.BRANCH_NAME}}/ \
            --output-diff ./outputs-diff/

      - name: Upload diff
        uses: actions/upload-artifact@v4
        with:
          name: diff
          path: outputs-diff/

      - name: Post diff as comment
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        run: |
          gh pr comment ${{ github.event.number }} --repo ${{ github.repository }} --body-file outputs-diff/diff.md --edit-last --create-if-none
# ...
````

See `.github/workflows/pull_request.yml` for a working example.
