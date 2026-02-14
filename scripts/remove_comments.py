#!/usr/bin/env python3

import os
import sys
import argparse
from typing import List, Tuple


class CommentRemover:
    def __init__(self, preserve_shebang=True):
        self.preserve_shebang = preserve_shebang
        self.processed_files = 0
        self.total_comments_removed = 0
        self.excluded_dirs = {"vendor", ".git", ".vscode", "__pycache__", "node_modules", "dist", "build"}

        self.ext_lang = {
            ".sh": "shell", ".bash": "shell", ".zsh": "shell", ".fish": "shell",
            ".go": "c_like",
            ".js": "c_like", ".mjs": "c_like", ".cjs": "c_like",
            ".ts": "c_like", ".tsx": "c_like", ".jsx": "c_like",
            ".c": "c_like", ".h": "c_like", ".cpp": "c_like", ".hpp": "c_like",
        }

    def get_lang_for_file(self, filename: str) -> str:
        for ext, lang in self.ext_lang.items():
            if filename.endswith(ext):
                return lang
        return ""

    def find_target_files(self, directory: str) -> List[Tuple[str, str]]:
        """Recursively find all target source files and return list of (path, lang)."""
        targets: List[Tuple[str, str]] = []
        for root, dirs, files in os.walk(directory):
            dirs[:] = [d for d in dirs if d not in self.excluded_dirs]

            for file in files:
                lang = self.get_lang_for_file(file)
                if lang:
                    targets.append((os.path.join(root, file), lang))
        return targets
    
    def should_preserve_comment(self, comment: str, line_number: int) -> bool:
        """Check if comment should be preserved (shebang, etc.)."""
        if not self.preserve_shebang:
            return False
            
        if line_number == 0 and comment.strip().startswith('#!'):
            return True

        return False
    
    def remove_comments_from_shell(self, content: str) -> Tuple[str, int]:
        """Remove comments from shell script source code while preserving string literals."""
        lines = content.split('\n')
        cleaned_lines = []
        comments_removed = 0
        
        for i, line in enumerate(lines):
            cleaned_line = ""
            j = 0
            in_single_quote = False
            in_double_quote = False
            
            while j < len(line):
                char = line[j]
                
                if char == "'" and not in_double_quote:
                    if not in_single_quote:
                        in_single_quote = True
                    else:
                        in_single_quote = False
                    cleaned_line += char
                    j += 1
                    continue
                
                if char == '"' and not in_single_quote:
                    if not in_double_quote:
                        in_double_quote = True
                    elif j > 0 and line[j-1] != '\\':
                        in_double_quote = False
                    cleaned_line += char
                    j += 1
                    continue
                
                if in_double_quote and char == '\\' and j + 1 < len(line):
                    cleaned_line += char + line[j + 1]
                    j += 2
                    continue
                
                if in_single_quote or in_double_quote:
                    cleaned_line += char
                    j += 1
                    continue
                
                if char == '#':
                    comment_content = line[j:]
                    if not self.should_preserve_comment(comment_content, i):
                        comments_removed += 1
                        break
                    else:
                        cleaned_line += comment_content
                        break
                
                cleaned_line += char
                j += 1
            
            cleaned_line = cleaned_line.rstrip()
            cleaned_lines.append(cleaned_line)
        
        return '\n'.join(cleaned_lines), comments_removed

    def remove_comments_from_c_like(self, content: str) -> Tuple[str, int]:
        """Remove // line and /* */ block comments while preserving string/char/backtick literals.

        Limitations:
        - Does not parse JS template interpolation (${...}) specially; comments inside interpolation will be preserved.
        - Does not attempt to distinguish JS regex literals from division operators.
        These are acceptable for many codebases; adjust if needed.
        """
        i = 0
        n = len(content)
        out_chars: List[str] = []
        comments_removed = 0

        in_single = False   # '
        in_double = False   # "
        in_backtick = False # ` (Go raw string or JS template literal)
        in_line_comment = False  # // ... \n
        in_block_comment = False # /* ... */

        while i < n:
            ch = content[i]
            nxt = content[i+1] if i + 1 < n else ''

            if in_line_comment:
                if ch == '\n':
                    in_line_comment = False
                    out_chars.append(ch)
                i += 1
                continue

            if in_block_comment:
                if ch == '*' and nxt == '/':
                    in_block_comment = False
                    i += 2
                    continue
                i += 1
                continue

            if in_single:
                out_chars.append(ch)
                if ch == '\\':
                    if i + 1 < n:
                        out_chars.append(content[i+1])
                        i += 2
                        continue
                elif ch == "'":
                    in_single = False
                i += 1
                continue

            if in_double:
                out_chars.append(ch)
                if ch == '\\':
                    if i + 1 < n:
                        out_chars.append(content[i+1])
                        i += 2
                        continue
                elif ch == '"':
                    in_double = False
                i += 1
                continue

            if in_backtick:
                out_chars.append(ch)
                if ch == '\\' and i + 1 < n and content[i+1] == '`':
                    out_chars.append('`')
                    i += 2
                    continue
                if ch == '`':
                    in_backtick = False
                i += 1
                continue

            if ch == '/' and nxt == '/':
                in_line_comment = True
                comments_removed += 1
                i += 2
                continue
            if ch == '/' and nxt == '*':
                in_block_comment = True
                comments_removed += 1
                i += 2
                continue

            if ch == "'":
                in_single = True
                out_chars.append(ch)
                i += 1
                continue
            if ch == '"':
                in_double = True
                out_chars.append(ch)
                i += 1
                continue
            if ch == '`':
                in_backtick = True
                out_chars.append(ch)
                i += 1
                continue

            out_chars.append(ch)
            i += 1

        return ''.join(out_chars), comments_removed
    
    def process_file(self, filepath: str, lang: str, dry_run: bool = False) -> bool:
        """
        Process a single source file to remove comments.
        Returns True if file was modified, False otherwise.
        """
        try:
            with open(filepath, 'r', encoding='utf-8') as f:
                original_content = f.read()
        except Exception as e:
            print(f"Error reading {filepath}: {e}")
            return False

        if lang == 'shell':
            cleaned_content, comments_removed = self.remove_comments_from_shell(original_content)
        elif lang == 'c_like':
            cleaned_content, comments_removed = self.remove_comments_from_c_like(original_content)
        else:
            return False
        
        if comments_removed == 0:
            return False
        
        if dry_run:
            print(f"Would remove {comments_removed} comments from {filepath}")
            return True
        
        try:
            with open(filepath, 'w', encoding='utf-8') as f:
                f.write(cleaned_content)
            print(f"Removed {comments_removed} comments from {filepath}")
            self.total_comments_removed += comments_removed
            return True
        except Exception as e:
            print(f"Error writing {filepath}: {e}")
            return False
    
    def process_directory(self, directory: str, dry_run: bool = False, remove_empty_dirs: bool = True) -> None:
        """Process all supported source files in directory, optionally removing empty directories."""
        if not os.path.isdir(directory):
            print(f"Error: {directory} is not a directory")
            return

        targets = self.find_target_files(directory)

        if not targets:
            print(f"No supported source files found in {directory}")
            return

        print(f"Found {len(targets)} files to process")
        
        if dry_run:
            print("DRY RUN - No files will be modified")
        
        modified_files = 0

        for filepath, lang in targets:
            if self.process_file(filepath, lang, dry_run):
                modified_files += 1
            self.processed_files += 1
        
        print("\nSummary:")
        print(f"  Files processed: {self.processed_files}")
        print(f"  Files modified: {modified_files}")
        if not dry_run:
            print(f"  Total comments removed: {self.total_comments_removed}")
        
        if remove_empty_dirs:
            removed_dirs = self.remove_empty_directories(directory, dry_run=dry_run)
            print(f"  Empty directories removed: {removed_dirs}")

    def remove_empty_directories(self, root_dir: str, dry_run: bool = False) -> int:
        """Remove empty directories under root_dir (post-order), excluding certain directories.

        Returns the count of directories removed (or that would be removed in dry-run).
        """
        removed_count = 0
        for current_root, dirs, files in os.walk(root_dir, topdown=False):
            for d in dirs:
                dir_path = os.path.join(current_root, d)
                if d in self.excluded_dirs:
                    continue
                rel = os.path.relpath(dir_path, root_dir)
                parts = rel.split(os.sep)
                if any(part in self.excluded_dirs for part in parts):
                    continue
                try:
                    if os.path.isdir(dir_path) and not os.listdir(dir_path):
                        if dry_run:
                            print(f"Would remove empty directory: {dir_path}")
                        else:
                            os.rmdir(dir_path)
                            print(f"Removed empty directory: {dir_path}")
                        removed_count += 1
                except OSError as e:
                    print(f"Could not remove directory {dir_path}: {e}")
        return removed_count


def main():
    parser = argparse.ArgumentParser(
        description="Remove comments from shell script files and clean up empty directories",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  python remove_comments.py                    # Process current directory
  python remove_comments.py /path/to/project   # Process specific directory
  python remove_comments.py --dry-run          # See what would be changed
  python remove_comments.py --no-preserve      # Remove all comments including shebang
  python remove_comments.py --keep-empty-dirs  # Do not remove empty directories
        """
    )

    parser.add_argument(
        'directory',
        nargs='?',
        default='.',
        help='Directory to process (default: current directory)'
    )

    parser.add_argument(
        '--dry-run',
        action='store_true',
        help='Show what would be changed without modifying files'
    )

    parser.add_argument(
        '--no-preserve',
        action='store_true',
        help='Remove all comments including shebang lines'
    )

    parser.add_argument(
        '--keep-empty-dirs',
        action='store_true',
        help='Do not remove empty directories after processing'
    )

    args = parser.parse_args()

    directory = os.path.abspath(args.directory)

    if not os.path.exists(directory):
        print(f"Error: Directory {directory} does not exist")
        sys.exit(1)

    print(f"Processing shell script files in: {directory}")

    remover = CommentRemover(preserve_shebang=not args.no_preserve)

    try:
        remover.process_directory(directory, dry_run=args.dry_run, remove_empty_dirs=not args.keep_empty_dirs)
    except KeyboardInterrupt:
        print("\nOperation cancelled by user")
        sys.exit(1)
    except Exception as e:
        print(f"Error: {e}")
        sys.exit(1)


if __name__ == "__main__":
    main()