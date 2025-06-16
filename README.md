# This tool can extract, update, and list files in `.DAT` Higurashi Daybreak files

## Usage:

**GUI Mode (default):**  
```bash
BundleTools.exe
# or
BundleTools.exe <datfile>
# or
BundleTools.exe -gui
```

**Command Line Mode:**

**Listing files inside the .DAT:**  
```bash
BundleTools.exe <datfile> -list
```

**Extracting files:**  
```bash
BundleTools.exe <datfile> -extract <output_folder>
# or with file pattern filter
BundleTools.exe <datfile> -extract <output_folder> -pattern <files_pattern>
```

**Updating/Patching from source directory:**  
```bash
BundleTools.exe <datfile> -update <source_files_path>
```

**Patching a single file:**  
```bash
BundleTools.exe <datfile> -single-patch <input_file>:<index>
```

> **Note:** Update and patch operations create backups of the original .DAT file before patching.

> ⚠️ Not finished and barely tested!

# How to Convert Daybreak `.X` Files to GLTF

1. **Open the Model in Fragmotion**  
   Download Fragmotion from: [http://www.fragmosoft.com/fragMOTION/index.php](http://www.fragmosoft.com/fragMOTION/index.php)  
   > ⚠️ Fragmotion may crash multiple times before successfully loading a model — just keep trying.

2. **(Optional) Remove Textures**  
   In the *Textures* tab, delete all textures.  
   > This step is recommended because Japanese characters in texture names can cause Blender not be able to import the files.

3. **Export as .X (Text Format)**  
   - Go to **File > Export**  
   - Choose the file type: `.x`  
   - Make sure the export is in **text format**, not binary.

4. **Fix Texture Filenames (if needed)**  
   If you didn’t remove the textures earlier, open the exported `.x` file in a text editor.  
   - Find all texture file names.  
   - Rename them to something that doesn't include Japanese characters.  
   - You can automate this step with a simple script.

5. **Convert `.X` to `.glTF` Using Assimp**  
   Install assimp from https://github.com/assimp/assimp/releases
   Use the following command:  
   ```bash
   assimp.exe export OriginalModel.x OutputModel.gltf
   ```

6. **Import into Blender**  
   You can now import the `.gltf` file into Blender.  
   - The model will include animations in the NLA tracks.  
   - However, animation timing may be incorrect (often too slow).  
   - You’ll need to adjust the keyframes manually in Blender.
