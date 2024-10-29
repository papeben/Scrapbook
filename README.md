# Scrapbook
Self-hosted website editor with friendly user interface and common components for quick deployment.

# Features
- Password protected edit mode
- Multiple pages
- Save to JSON (from UI and backend)
- Image & video upload 
- Automatic media optimization
- CSS editor
- Docker deployment
- Deb package
- Windows exe
- Embedding custom HTML components
- Markdown text rendering

# Components
- Header
- Navigation bar
- Social media embeds
- Countdown

# Software Design

## Page design

- Page structure broken up into rows, cells, and contents.
    - Rows are horizontal divisions which cells sit inside of.
    - Cells are the boxes inside rows which contain content.
    - Contents are text, images, video, or other content.

## Edit mode 

- Password protected edit mode
- Add rows and cells
- Colour picker for background and text
- Background image support
- CSS element edit UI
