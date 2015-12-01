//----------- TAB SELECTION ------------//
$(function() {
  $('.tab-panels .tabs li').on('click', function() {

    //On mouse click find the next element of tab panels
    //find the closes active one and remove that class
    //set the current panel to active
    var $panel = $(this).closest('.tab-panels');
    $panel.find('.tabs li.active').removeClass('active');
    $(this).addClass('active');

    //find which panel to show
    //The related panel is saved in the list item as an attribute
    var panelToShow = $(this).attr('rel');

    //Hide the current panel
    $panel.find('.panel.active').fadeOut(200, showNextPanel);

    //show next panel
  
    function showNextPanel() {
      $(this).removeClass('active');

      $('#'+panelToShow).fadeIn(200, function() {
        $(this).addClass('active');
      });
    }

  });

});

//---------- SUBMIT BUTTON RELOAD PAGE ----------- //

$(function() {
  $('#submitButton').on('click', function() {
  
    //On mouse click reload the page
    location.reload(); 
  });
});
