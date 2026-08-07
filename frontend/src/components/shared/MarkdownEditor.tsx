import { onMount, onCleanup, Component, Show } from "solid-js";
import "easymde/dist/easymde.min.css";
import "./MarkdownEditor.css";
import { MarkdownLogo, Image } from "~/components/icons/Phosphor";
import type EasyMDE from "easymde";

interface MarkdownEditorProps {
  initialValue?: string;
  onValueChange?: (value: string) => void;
  placeholder?: string;
  onCancel?: () => void;
  onSubmit?: () => void;
}

export const MarkdownEditor: Component<MarkdownEditorProps> = (props) => {
  let textareaRef: HTMLTextAreaElement | undefined;
  let easymde: EasyMDE | undefined;

  // onMount only runs in the browser, AFTER the server has rendered the page.
  onMount(async () => {
    if (textareaRef) {
      // We dynamically import the library ONLY on the client.
      const EasyMDE = (await import("easymde")).default;
      const EasyMDECSS = (await import("easymde/dist/easymde.min.css?inline"))
        .default;

      // Inject styles into the document's head
      const style = document.createElement("style");
      style.innerHTML = EasyMDECSS;
      document.head.appendChild(style);

      easymde = new EasyMDE({
        element: textareaRef,
        initialValue: props.initialValue || "",
        placeholder:
          props.placeholder ||
          "Ask a question, start a conversation, or make an announcement",
        spellChecker: false,
        toolbar: [
          "heading",
          "bold",
          "italic",
          "|",
          "quote",
          "code",
          "link",
          "|",
          "unordered-list",
          "ordered-list",
          "strikethrough",
          "|",
          "preview",
          "side-by-side",
        ],
        status: false,
      });

      easymde.codemirror.on("change", () => {
        if (props.onValueChange) {
          props.onValueChange(easymde?.value() || "");
        }
      });
    }
  });

  onCleanup(() => {
    easymde?.toTextArea();
    easymde = undefined;
  });

  return (
    <div class="easymde-container">
      <textarea
        ref={textareaRef}
        class="initial-textarea"
        placeholder={props.placeholder}
      />
      {/* --- Custom Footer --- */}
      <div class="editor-footer">
        <div class="footer-actions-group">
          <a
            href="https://docs.github.com/en/get-started/writing-on-github/getting-started-with-writing-and-formatting-on-github/basic-writing-and-formatting-syntax"
            target="_blank"
            rel="noopener noreferrer"
            class="footer-link"
          >
            <MarkdownLogo size={16} />
            <span>Markdown is supported</span>
          </a>
          <div class="footer-separator" />
          <button class="footer-button">
            <Image size={16} />
            <span>Paste, drop, or click to add images</span>
          </button>
        </div>
      </div>
      <Show when={props.onCancel || props.onSubmit}>
        <div class="editor-actions">
          <Show when={props.onCancel}>
            <button class="cancel-button" onClick={props.onCancel}>
              Cancel
            </button>
          </Show>
          <button class="submit-button" onClick={props.onSubmit}>
            Submit
          </button>
        </div>
      </Show>
    </div>
  );
};
